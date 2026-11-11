package workers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cybermedico/cmd/cybermedico/api/log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// These jobs are scheduled for the moment a doctor's deadline passes, in the
// same transaction that sets the deadline. If the doctor hasn't responded by
// then, the consultation is marked not attended.

// WrittenNotAttendedArgs: no answer to a written consultation within 72 hours.
type WrittenNotAttendedArgs struct {
	ConsultationID string `json:"consultation_id"`
}

// VideoUnscheduledNotAttendedArgs: a video call not scheduled in time.
type VideoUnscheduledNotAttendedArgs struct {
	ConsultationID string `json:"consultation_id"`
}

// VideoScheduledNotAttendedArgs: no opinion within 24 hours after the call.
type VideoScheduledNotAttendedArgs struct {
	ConsultationID string `json:"consultation_id"`
}

func (WrittenNotAttendedArgs) Kind() string { return "written_consultation_not_attended" }

func (VideoUnscheduledNotAttendedArgs) Kind() string {
	return "video_unscheduled_consultation_not_attended"
}

func (VideoScheduledNotAttendedArgs) Kind() string {
	return "video_scheduled_consultation_not_attended"
}

// NotAttendedJobOpts schedules a marker job for the deadline. Unique by its
// arguments, so a consultation gets one marker per deadline.
func NotAttendedJobOpts(deadline time.Time) *river.InsertOpts {
	return &river.InsertOpts{ScheduledAt: deadline, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// notAttendedCheck is the status the consultation waits in and the query for
// its deadline.
type notAttendedCheck struct {
	waitingStatus string
	deadlineQuery string
}

var (
	writtenNotAttended = notAttendedCheck{
		waitingStatus: "pending",
		deadlineQuery: `SELECT expires_at FROM written_consultations WHERE consultation_id = @id`,
	}
	videoUnscheduledNotAttended = notAttendedCheck{
		waitingStatus: "pending",
		deadlineQuery: `SELECT requires_response_by_schedule FROM video_consultations WHERE consultation_id = @id`,
	}
	videoScheduledNotAttended = notAttendedCheck{
		waitingStatus: "scheduled",
		deadlineQuery: `SELECT expires_at_after_video FROM video_consultations WHERE consultation_id = @id`,
	}
)

// markNotAttended marks the consultation if it still waits on the doctor and
// its deadline has passed.
func markNotAttended(ctx context.Context, db *pgxpool.Pool, logger *log.LoggerType, consultationID string, check notAttendedCheck) error {
	var deadline *time.Time
	err := db.QueryRow(ctx, check.deadlineQuery, pgx.NamedArgs{"id": consultationID}).Scan(&deadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // the consultation is gone
	}
	if err != nil {
		return fmt.Errorf("failed to query deadline: %w", err)
	}
	if deadline == nil {
		return nil
	}
	// The job runs at the deadline; if the clocks disagree by a moment, wait for it.
	if until := time.Until(*deadline); until > 0 {
		return river.JobSnooze(until + time.Second)
	}

	// The status condition makes this safe against the doctor responding at the
	// same moment: whichever update runs first wins, the other changes nothing.
	tag, err := db.Exec(ctx, `
		UPDATE consultations SET consultation_status = 'not_attended'
		WHERE id = @id AND consultation_status = @status
	`, pgx.NamedArgs{"id": consultationID, "status": check.waitingStatus})
	if err != nil {
		return fmt.Errorf("failed to mark consultation as not attended: %w", err)
	}

	if tag.RowsAffected() == 1 {
		logger.Info("consultation marked as not attended", "consultation_id", consultationID, "deadline", *deadline)
	}

	return nil
}

type WrittenNotAttendedWorker struct {
	river.WorkerDefaults[WrittenNotAttendedArgs]
	db     *pgxpool.Pool
	logger *log.LoggerType
}

func NewWrittenNotAttendedWorker(db *pgxpool.Pool, logger *log.LoggerType) *WrittenNotAttendedWorker {
	return &WrittenNotAttendedWorker{db: db, logger: logger}
}

func (w *WrittenNotAttendedWorker) Work(ctx context.Context, job *river.Job[WrittenNotAttendedArgs]) error {
	return markNotAttended(ctx, w.db, w.logger, job.Args.ConsultationID, writtenNotAttended)
}

type VideoUnscheduledNotAttendedWorker struct {
	river.WorkerDefaults[VideoUnscheduledNotAttendedArgs]
	db     *pgxpool.Pool
	logger *log.LoggerType
}

func NewVideoUnscheduledNotAttendedWorker(db *pgxpool.Pool, logger *log.LoggerType) *VideoUnscheduledNotAttendedWorker {
	return &VideoUnscheduledNotAttendedWorker{db: db, logger: logger}
}

func (w *VideoUnscheduledNotAttendedWorker) Work(ctx context.Context, job *river.Job[VideoUnscheduledNotAttendedArgs]) error {
	return markNotAttended(ctx, w.db, w.logger, job.Args.ConsultationID, videoUnscheduledNotAttended)
}

type VideoScheduledNotAttendedWorker struct {
	river.WorkerDefaults[VideoScheduledNotAttendedArgs]
	db     *pgxpool.Pool
	logger *log.LoggerType
}

func NewVideoScheduledNotAttendedWorker(db *pgxpool.Pool, logger *log.LoggerType) *VideoScheduledNotAttendedWorker {
	return &VideoScheduledNotAttendedWorker{db: db, logger: logger}
}

func (w *VideoScheduledNotAttendedWorker) Work(ctx context.Context, job *river.Job[VideoScheduledNotAttendedArgs]) error {
	return markNotAttended(ctx, w.db, w.logger, job.Args.ConsultationID, videoScheduledNotAttended)
}
