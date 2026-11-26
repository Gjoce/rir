# 001 – Razpoložljivost zdravnikov: naloge

**Specifikacija:** [spec.md](spec.md) · **Načrt:** [plan.md](plan.md)
**Veja:** `feature/001-doctor-availability`

Vsaka naloga ima svoje preverjanje. Commit sledi šele, ko so opravljene vse naloge v skupini in je zelena končna kontrola skupine:

```
mise exec -- golangci-lint run ./...
mise exec -- go test ./...
mise run test:policies
```

`.github/workflows/ci-cd.yml` (necommitana sprememba, ki ni del te naloge) ne gre v noben commit.

---

## Commit 1 – `feat: add change_doctor_availability policy`

- [x] **T1** V `authorization/policies_test/doctors_test.rego` dodaj testa `test_admin_changes_doctor_availability` (dovoljeno) in `test_doctor_cannot_change_doctor_availability` (zavrnjeno).
  *Preveri:* `mise run test:policies` – prvi test pade (pravila še ni), drugi je zelen.
- [x] **T2** Dodaj testa `test_anonymous_cannot_change_doctor_availability` in `test_unknown_role_cannot_change_doctor_availability` (`Role: "nurse"`).
  *Preveri:* `mise run test:policies` – oba zelena.
- [x] **T3** V `authorization/policies/doctors.rego` dodaj pravilo `allow`, kjer je `operation_id == "change_doctor_availability"` in `caller.Role == "admin"`.
  *Preveri:* `mise run test:policies` – vsi testi zeleni, tudi T1.
- [x] **T4** Dodaj `"change_doctor_availability"` v seznam `doctor_management_operations` v `doctors_test.rego`.
  *Preveri:* `mise run test:policies` zelen; `mise exec -- opa check authorization/policies authorization/policies_test` brez napak.
- [x] **T5** Končna kontrola (lint, Go testi, Rego testi), nato commit.

## Commit 2 – `feat: add admin doctors page with availability toggle`

- [x] **T6** V `cmd/cybermedico/openapi.yml` dodaj `GET /doctors` (`operationId: doctors_form`, `sessionCookie`, odgovora `200` in `default`).
  *Preveri:* `mise run generate` uspe; v `ogen/oas_server_gen.go` obstaja `DoctorsForm`.
- [x] **T7** Dodaj `POST /doctors/{id}/availability` (`operationId: doctor_availability_submit`, parameter `UUID`, telo `application/x-www-form-urlencoded` z obveznim `available: boolean`, odgovora `303` in `default`).
  *Preveri:* `mise run generate` uspe; obstajata `DoctorAvailabilitySubmit` in `DoctorAvailabilitySubmitReq` s poljem `Available bool`. `go build ./...` pade, ker manjka handler – to je pričakovano.
- [x] **T8** Ustvari `cmd/cybermedico/api/html/doctors.go`: struct `DoctorsHandler{authService, db}`, `NewDoctorsHandler`, `DoctorRow` (ID, DisplayName, Specialization, Workplace, Available).
  *Preveri:* datoteka se prevede (`go vet ./cmd/cybermedico/api/html/` brez napak v `doctors.go`).
- [x] **T9** Napiši `doctors_test.go` s tabelaričnima testoma `TestAvailabilityLabel` (`true` → *Available*, `false` → *Unavailable*) in `TestAvailabilityAction` (`true` → *Mark unavailable*/`"false"`, `false` → *Mark available*/`"true"`).
  *Preveri:* testa padeta, ker funkciji še ne obstajata.
- [x] **T10** V `doctors.go` implementiraj `availabilityLabel` in `availabilityAction`.
  *Preveri:* `go test ./cmd/cybermedico/api/html/ -run 'TestAvailability'` zelen.
- [x] **T11** Ustvari `doctors.templ` s `DoctorsPage(doctors []DoctorRow)` v `layouts.Dashboard(nil)`: naslov *Doctors*, tabela `striped` (*Name*, *Specialization*, *Workplace*, *Status*, *Action*), v vsaki vrstici obrazec `POST /doctors/{id}/availability` s skritim poljem `available` in gumbom; za prazen seznam *No doctors yet.*
  *Preveri:* `mise exec -- templ generate` uspe, nastane `doctors_templ.go`.
- [x] **T12** Dodaj `TestDoctorsPageRendersAllDoctors`: izris z enim razpoložljivim in enim nerazpoložljivim zdravnikom; preveri imena, specializaciji, delovni mesti, *Available*/*Unavailable*, `action="/doctors/{id}/availability"` in `value="false"`/`value="true"`. Dodaj še primer s praznim seznamom (*No doctors yet.*).
  *Preveri:* `go test ./cmd/cybermedico/api/html/ -run TestDoctorsPage` zelen.
- [x] **T13** Implementiraj `DoctorsForm`: najprej `IsAllowed("doctors_form")` (ob zavrnitvi 403, `ButtonLink: "/dashboard"`), nato `SELECT id, display_name, specialization, workplace, available FROM doctor_profile ORDER BY display_name, id`, `CollectRows`, izris v `R200`. Napake ovij s `fmt.Errorf`.
  *Preveri:* pregled kode: preverjanje dovoljenja je pred poizvedbo, SQL nima lepljenja nizov.
- [x] **T14** Implementiraj `DoctorAvailabilitySubmit`: najprej `IsAllowed("change_doctor_availability")` (403), nato `UPDATE doctor_profile SET available = @available WHERE id = @id`; `RowsAffected() == 0` → 404 *Doctor not found*; sicer `R303` na `/doctors`.
  *Preveri:* pregled kode: `UPDATE` se dotakne samo tabele `doctor_profile` (AC8).
- [x] **T15** V `cmd/cybermedico/main.go` dodaj `html.DoctorsHandler` v vgrajeno strukturo in `html.NewDoctorsHandler(authService, dbPool)` v `initializeHTMLServer`.
  *Preveri:* `mise exec -- go build ./...` uspe.
- [x] **T16** Ročni dim test (`mise run develop`, prijava kot admin): `/doctors` prikaže 4 zdravnike po abecedi, vsi *Available*.
- [x] **T17** Končna kontrola (lint, Go testi, Rego testi), nato commit (`openapi.yml`, `ogen/`, `doctors.go`, `doctors.templ`, `doctors_templ.go`, `doctors_test.go`, `main.go`).

## Commit 3 – `feat: show Doctors link in sidebar for admins`

- [x] **T18** V `cmd/cybermedico/api/util/util_test.go` dodaj tabelarični `TestGetRole` (kontekst z vlogo `admin`, z vlogo `doctor`, brez vloge → `""`).
  *Preveri:* test pade, ker `GetRole` še ne obstaja.
- [x] **T19** V `cmd/cybermedico/api/util/httputil.go` dodaj `GetRole(ctx)`, ki bere `UserRoleContextKey` (vzorec `GetUserID`).
  *Preveri:* `go test ./cmd/cybermedico/api/util/` zelen.
- [x] **T20** V `components/sidebar.templ` med *Overview* in *Content* dodaj razdelek *Management* s povezavo *Doctors* (`/doctors`, `ClassIfActive`/`RoleIfActive`, ikona), samo kadar je `util.GetRole(ctx) == "admin"`.
  *Preveri:* `mise exec -- templ generate` uspe, posodobljen je `sidebar_templ.go`.
- [x] **T21** Ročno: admin vidi *Doctors* in je ta poudarjen na `/doctors`; dr. Carter povezave ne vidi.
- [x] **T22** Končna kontrola (lint, Go testi, Rego testi), nato commit.

## Commit 4 – `test: cover /doctors login redirect in hurl`

- [ ] **T23** V `tests/hurl/doctors.hurl` dodaj primer s komentarjem: `GET {{host}}/doctors` brez prijave → `HTTP 303`, `Location == "/auth/login"` (AC6).
  *Preveri:* `mise run test:hurl` (aplikacija teče) – primer zelen.
- [ ] **T24** Dodaj primer: `POST {{host}}/doctors/22222222-2222-2222-2222-222222222221/availability` z `[Form] available: false` brez prijave → `303` na `/auth/login` (AC6).
  *Preveri:* `mise run test:hurl` zelen; javni seznam v istem testu še vedno vsebuje *Dr. Emily Carter* (zahtevek ni ničesar spremenil).
- [ ] **T25** Končna kontrola (lint, Go testi, Rego testi, `mise run test:hurl`), nato commit.

## Končno ročno preverjanje (po vseh commitih)

Prijava `admin@cybermedico.com` + TOTP, `mise run develop`:

- [ ] **T26** *Mark unavailable* pri dr. Carter → preusmeritev na `/doctors`, status *Unavailable*, gumb *Mark available* (AC2).
- [ ] **T27** Odjavljen: `/` ne prikaže dr. Carter; `/specialists/22222222-2222-2222-2222-222222222221`, `/written` in `/video` vrnejo 404 (AC3).
- [ ] **T28** `/consultations` za dr. Carter je enak kot pred preklopom (AC8).
- [ ] **T29** `POST /doctors/<naključen UUID>/availability` kot admin → 404 (AC7).
- [ ] **T30** *Mark available* → dr. Carter je spet na `/` (AC4).
- [ ] **T31** Prijava `dr.carter@cybermedico.com`: `/doctors` in `POST` na preklop vrneta 403 (AC5).
- [ ] **T32** Stanje vrnjeno: vsi zdravniki *Available*.
