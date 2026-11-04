package html

import (
	"bytes"
	"context"
	herrors "cybermedico/cmd/cybermedico/api/errors"
	"cybermedico/cmd/cybermedico/api/log"
	api "cybermedico/cmd/cybermedico/ogen"
	"cybermedico/cmd/cybermedico/storage"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	ht "github.com/ogen-go/ogen/http"
)

const (
	maxOrderNameLength  = 100
	maxOrderEmailLength = 254
	maxOrderPhoneLength = 30
	maxOrderTextLength  = 5000
	maxPatientAge       = 120

	maxAttachmentSize     = 10 * 1024 * 1024 // 10 MB
	maxAttachmentCount    = 5
	maxAttachmentNameSize = 255
)

var attachmentExtensions = map[string]string{
	"application/pdf": ".pdf",
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
}

type OrdersHandler struct {
	db      *pgxpool.Pool
	storage *storage.Service
}

func NewOrdersHandler(db *pgxpool.Pool, storageService *storage.Service) OrdersHandler {
	return OrdersHandler{db: db, storage: storageService}
}

type orderAttachment struct {
	file     io.Reader
	s3Key    string
	filename string
	mimeType string
	size     int64
}

// validateAttachments checks every file before anything is uploaded. It returns a
// message for the form when a file is refused.
func validateAttachments(files []ht.MultipartFile) ([]orderAttachment, string) {
	if len(files) > maxAttachmentCount {
		return nil, fmt.Sprintf("You can attach at most %d files. Choose your files again.", maxAttachmentCount)
	}

	attachments := make([]orderAttachment, 0, len(files))
	for _, f := range files {
		// Keep only the file's own name, some clients send a full path.
		filename := path.Base(strings.ReplaceAll(f.Name, `\`, "/"))
		if utf8.RuneCountInString(filename) > maxAttachmentNameSize {
			filename = string([]rune(filename)[:maxAttachmentNameSize])
		}

		if f.Size > maxAttachmentSize {
			return nil, fmt.Sprintf("%q is larger than 10 MB. Choose your files again.", filename)
		}

		// Sniff the actual content type from the first 512 bytes — do not trust the browser header.
		sniffBuf := make([]byte, 512)
		n, err := io.ReadFull(f.File, sniffBuf)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Sprintf("%q could not be read. Choose your files again.", filename)
		}

		mimeType, _, _ := mime.ParseMediaType(http.DetectContentType(sniffBuf[:n]))
		if _, allowed := attachmentExtensions[mimeType]; !allowed {
			return nil, fmt.Sprintf("%q must be a PDF, JPG or PNG file. Choose your files again.", filename)
		}

		attachments = append(attachments, orderAttachment{
			file:     io.MultiReader(bytes.NewReader(sniffBuf[:n]), f.File),
			filename: filename,
			mimeType: mimeType,
			size:     f.Size,
		})
	}

	return attachments, ""
}

// uploadAttachments stores the files and fills in their keys. Attachments that were
// uploaded stay in the slice even on error, so the caller can delete them.
func (h OrdersHandler) uploadAttachments(ctx context.Context, consultationID string, attachments []orderAttachment) error {
	for i := range attachments {
		a := &attachments[i]

		// The S3 client needs a seekable body to retry, so the file is buffered.
		data, err := io.ReadAll(io.LimitReader(a.file, maxAttachmentSize))
		if err != nil {
			return fmt.Errorf("failed to read attachment %q: %w", a.filename, err)
		}
		a.size = int64(len(data))

		// The name in storage is generated: the patient's filename is never part of a path.
		key := fmt.Sprintf("consultations/%s/%s%s", consultationID, uuid.New().String(), attachmentExtensions[a.mimeType])
		if err := h.storage.Upload(ctx, key, bytes.NewReader(data), a.size, a.mimeType); err != nil {
			return fmt.Errorf("failed to upload attachment: %w", err)
		}
		a.s3Key = key
	}

	return nil
}

func (h OrdersHandler) deleteAttachments(ctx context.Context, attachments []orderAttachment) {
	// Still has to run when the request that failed was cancelled.
	ctx = context.WithoutCancel(ctx)
	for _, a := range attachments {
		if a.s3Key == "" {
			continue
		}
		if err := h.storage.Delete(ctx, a.s3Key); err != nil {
			log.Logger.Error("failed to delete orphaned S3 object", "key", a.s3Key, "error", err)
		}
	}
}

type orderDoctor struct {
	ID             string
	DisplayName    string
	Specialization string
	Price          string
}

type consultationOrder struct {
	PatientName    string
	PatientSurname string
	PatientAge     string
	PatientEmail   string
	PatientPhone   string
	MedicalProblem string
	Diagnosis      string
	Medication     string
	Allergies      string
	NeedTranslator bool
	TermsAccepted  bool

	PatientNameErr    string
	PatientSurnameErr string
	PatientAgeErr     string
	PatientEmailErr   string
	PatientPhoneErr   string
	MedicalProblemErr string
	DiagnosisErr      string
	MedicationErr     string
	AllergiesErr      string
	TermsAcceptedErr  string
	AttachmentsErr    string
}

func (o consultationOrder) Valid() bool {
	return o.PatientNameErr == "" && o.PatientSurnameErr == "" && o.PatientAgeErr == "" &&
		o.PatientEmailErr == "" && o.PatientPhoneErr == "" && o.MedicalProblemErr == "" &&
		o.DiagnosisErr == "" && o.MedicationErr == "" && o.AllergiesErr == "" && o.TermsAcceptedErr == "" && o.AttachmentsErr == ""
}

// validate fills the error fields. The name and phone can't contain line breaks
// because they end up in email headers, and the form is open to anyone.
func (o *consultationOrder) validate() {
	singleLine := func(label, value string, maxLength int) string {
		switch {
		case value == "":
			return label + " is required"
		case utf8.RuneCountInString(value) > maxLength:
			return fmt.Sprintf("%s can have at most %d characters", label, maxLength)
		case strings.ContainsAny(value, "\r\n"):
			return label + " can't contain line breaks"
		}

		return ""
	}
	optionalText := func(label, value string) string {
		if utf8.RuneCountInString(value) > maxOrderTextLength {
			return fmt.Sprintf("%s can have at most %d characters", label, maxOrderTextLength)
		}

		return ""
	}

	o.PatientNameErr = singleLine("Name", o.PatientName, maxOrderNameLength)
	o.PatientSurnameErr = singleLine("Surname", o.PatientSurname, maxOrderNameLength)
	o.PatientPhoneErr = singleLine("Phone", o.PatientPhone, maxOrderPhoneLength)

	if age, err := strconv.Atoi(o.PatientAge); o.PatientAge == "" {
		o.PatientAgeErr = "Age is required"
	} else if err != nil || age < 0 || age > maxPatientAge {
		o.PatientAgeErr = fmt.Sprintf("Age must be a number between 0 and %d", maxPatientAge)
	}

	if o.PatientEmail == "" {
		o.PatientEmailErr = "Email is required"
	} else if address, err := mail.ParseAddress(o.PatientEmail); err != nil || address.Address != o.PatientEmail || len(o.PatientEmail) > maxOrderEmailLength {
		o.PatientEmailErr = "Enter a valid email address"
	}

	if o.MedicalProblem == "" {
		o.MedicalProblemErr = "Description of the problem is required"
	} else {
		o.MedicalProblemErr = optionalText("Description of the problem", o.MedicalProblem)
	}
	o.DiagnosisErr = optionalText("Diagnosis", o.Diagnosis)
	o.MedicationErr = optionalText("Medication", o.Medication)
	o.AllergiesErr = optionalText("Allergies", o.Allergies)

	if !o.TermsAccepted {
		o.TermsAcceptedErr = "You have to accept the terms of use"
	}
}

// loadOrderDoctor returns the doctor with the price for the given type of
// consultation. Doctors who are not available can't be ordered from.
func (h OrdersHandler) loadOrderDoctor(ctx context.Context, doctorID uuid.UUID, consultationType string) (orderDoctor, error) {
	rows, err := h.db.Query(ctx, `
		SELECT
			id::text AS id,
			display_name,
			specialization,
			(CASE WHEN @consultation_type = 'video' THEN consultation_price_video ELSE consultation_price_written END)::text AS price
		FROM doctor_profile
		WHERE id = @id AND available
	`, pgx.NamedArgs{"id": doctorID.String(), "consultation_type": consultationType})
	if err != nil {
		return orderDoctor{}, fmt.Errorf("failed to query doctor: %w", err)
	}

	doctor, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[orderDoctor])
	if errors.Is(err, pgx.ErrNoRows) {
		return orderDoctor{}, herrors.HtmlError{Code: http.StatusNotFound, Msg: "This doctor is not available.", ButtonText: "Show all doctors", ButtonLink: "/"}
	}
	if err != nil {
		return orderDoctor{}, fmt.Errorf("failed to collect doctor: %w", err)
	}

	return doctor, nil
}

func (h OrdersHandler) WrittenOrderForm(ctx context.Context, params api.WrittenOrderFormParams) (api.WrittenOrderFormRes, error) {
	doctor, err := h.loadOrderDoctor(ctx, params.ID, "written")
	if err != nil {
		return nil, err
	}

	var buff bytes.Buffer

	return &api.R200{Data: &buff}, WrittenOrderHtml(doctor, consultationOrder{}).Render(ctx, &buff)
}

func (h OrdersHandler) WrittenOrderSubmit(ctx context.Context, request *api.WrittenOrderSubmitReq, params api.WrittenOrderSubmitParams) (api.WrittenOrderSubmitRes, error) {
	doctor, err := h.loadOrderDoctor(ctx, params.ID, "written")
	if err != nil {
		return nil, err
	}

	order := consultationOrder{
		PatientName:    strings.TrimSpace(request.PatientName.Or("")),
		PatientSurname: strings.TrimSpace(request.PatientSurname.Or("")),
		PatientAge:     strings.TrimSpace(request.PatientAge.Or("")),
		PatientEmail:   strings.TrimSpace(request.PatientEmail.Or("")),
		PatientPhone:   strings.TrimSpace(request.PatientPhone.Or("")),
		MedicalProblem: strings.TrimSpace(request.MedicalProblem.Or("")),
		Diagnosis:      strings.TrimSpace(request.PatientDiagnosis.Or("")),
		Medication:     strings.TrimSpace(request.Medication.Or("")),
		Allergies:      strings.TrimSpace(request.Allergies.Or("")),
		NeedTranslator: request.NeedTranslator.Or(false),
		TermsAccepted:  request.TermsAccepted.Or(false),
	}
	order.validate()

	attachments, attachmentsErr := validateAttachments(request.Attachments)
	order.AttachmentsErr = attachmentsErr

	if !order.Valid() {
		// A browser can't keep chosen files when the form is shown again.
		if order.AttachmentsErr == "" && len(request.Attachments) > 0 {
			order.AttachmentsErr = "Your files were not saved. Choose them again."
		}

		var buff bytes.Buffer

		return &api.R200{Data: &buff}, WrittenOrderHtml(doctor, order).Render(ctx, &buff)
	}

	consultationID := uuid.New().String()

	// Files go to storage before the transaction opens, so a slow upload doesn't hold
	// a database connection. If anything fails later, the defer removes them again.
	committed := false
	defer func() {
		if !committed {
			h.deleteAttachments(ctx, attachments)
		}
	}()

	if err := h.uploadAttachments(ctx, consultationID, attachments); err != nil {
		return nil, err
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Logger.Error("failed to rollback transaction", "error", err)
		}
	}()

	// The price comes from the doctor's profile, never from the form.
	tag, err := tx.Exec(ctx, `
		INSERT INTO consultations (
			id, doctor_id, consultation_type, consultation_status, consultation_payment_status,
			price, need_translator, terms_accepted,
			patient_name, patient_surname, patient_age, patient_phone, patient_email,
			medical_problem, patient_diagnosis, allergies, medication
		)
		SELECT
			@id, p.id, 'written', 'pending', 'pending',
			p.consultation_price_written, @need_translator, @terms_accepted,
			@patient_name, @patient_surname, @patient_age, @patient_phone, @patient_email,
			@medical_problem, NULLIF(@patient_diagnosis, ''), NULLIF(@allergies, ''), NULLIF(@medication, '')
		FROM doctor_profile p
		WHERE p.id = @doctor_id AND p.available
	`, pgx.NamedArgs{
		"id":                consultationID,
		"doctor_id":         doctor.ID,
		"need_translator":   order.NeedTranslator,
		"terms_accepted":    order.TermsAccepted,
		"patient_name":      order.PatientName,
		"patient_surname":   order.PatientSurname,
		"patient_age":       order.PatientAge,
		"patient_phone":     order.PatientPhone,
		"patient_email":     order.PatientEmail,
		"medical_problem":   order.MedicalProblem,
		"patient_diagnosis": order.Diagnosis,
		"allergies":         order.Allergies,
		"medication":        order.Medication,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to insert consultation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, herrors.HtmlError{Code: http.StatusNotFound, Msg: "This doctor is not available.", ButtonText: "Show all doctors", ButtonLink: "/"}
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO written_consultations (consultation_id) VALUES (@consultation_id)`,
		pgx.NamedArgs{"consultation_id": consultationID},
	); err != nil {
		return nil, fmt.Errorf("failed to insert consultation details: %w", err)
	}

	for _, a := range attachments {
		if _, err = tx.Exec(ctx, `
			INSERT INTO consultation_attachments (consultation_id, s3_key, original_filename, content_type, size_bytes)
			VALUES (@consultation_id, @s3_key, @original_filename, @content_type, @size_bytes)
		`, pgx.NamedArgs{
			"consultation_id":   consultationID,
			"s3_key":            a.s3Key,
			"original_filename": a.filename,
			"content_type":      a.mimeType,
			"size_bytes":        a.size,
		}); err != nil {
			return nil, fmt.Errorf("failed to insert attachment: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit consultation: %w", err)
	}
	committed = true

	return &api.R303{Location: api.NewOptString("/order-submitted")}, nil
}

func (h OrdersHandler) OrderSubmitted(ctx context.Context) (api.OrderSubmittedRes, error) {
	var buff bytes.Buffer

	return &api.R200{Data: &buff}, OrderSubmittedHtml().Render(ctx, &buff)
}
