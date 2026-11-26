# 001 – Razpoložljivost zdravnikov: tehnični načrt

**Specifikacija:** [spec.md](spec.md)
**Status:** načrt (koda še ni spremenjena)

## Povzetek pristopa

Nov `DoctorsHandler` z dvema operacijama:

1. `GET /doctors` – seznam vseh zdravnikov za administratorja (stran v `doctors.templ`, postavitev `layouts.Dashboard`).
2. `POST /doctors/{id}/availability` – navaden HTML obrazec (gumb) z eksplicitno ciljno vrednostjo `available=true|false`; po uspehu `303` nazaj na `/doctors` (vzorec Post/Redirect/Get, enako kot `ConsultationScheduleSubmit`).

Odločitve:

- **Eksplicitna vrednost namesto preklopa (`NOT available`)**: dvojni klik ali dva zavihka ne obrneta stanja dvakrat; zahtevek je idempotenten.
- **Navaden `<form method="POST">` namesto Datastar `@post`**: AC2/AC4 govorita o stanju "po osvežitvi"; PRG je najpreprostejši, deluje brez JS in ga lahko Hurl testira enako kot `/consultations/{id}/schedule`.
- **Brez nove migracije**: uporabi se `doctor_profile.available`; `updated_at` posodobi obstoječi sprožilec.
- **Brez spremembe javnih strani**: `root.go`, `specialists.go` in `orders.go` že filtrirajo z `available` (AC3 je obstoječe obnašanje).

## Datoteke, ki se spremenijo

| Datoteka | Sprememba |
|---|---|
| `cmd/cybermedico/openapi.yml` | dve novi poti (spodaj) |
| `cmd/cybermedico/ogen/*` | **samo** z `mise run generate` |
| `cmd/cybermedico/api/html/doctors.go` | **nova**: `DoctorsHandler`, `DoctorsForm`, `DoctorAvailabilitySubmit`, pomožni funkciji |
| `cmd/cybermedico/api/html/doctors.templ` + `doctors_templ.go` | **nova** stran; `_templ.go` samo z `templ generate` |
| `cmd/cybermedico/api/html/doctors_test.go` | **nova**: Go testi |
| `cmd/cybermedico/main.go` | `html.DoctorsHandler` v vgrajeno strukturo in `html.NewDoctorsHandler(authService, dbPool)` v `initializeHTMLServer` |
| `cmd/cybermedico/api/util/httputil.go` | nova `GetRole(ctx)` (bere obstoječi `UserRoleContextKey`, ki ga že nastavi `SessionMiddleware`) |
| `cmd/cybermedico/api/util/util_test.go` | test za `GetRole` |
| `components/sidebar.templ` + `sidebar_templ.go` | razdelek *Management* s povezavo **Doctors** samo za admina |
| `authorization/policies/doctors.rego` | novo pravilo `change_doctor_availability` |
| `authorization/policies_test/doctors_test.rego` | testi za novo pravilo |
| `tests/hurl/doctors.hurl` | primeri za neprijavljenega uporabnika |

**Ne spreminjam:** migracij, `resources/config/` (seed), `.github/workflows/`, `go.mod`/`mise.toml`. Nove odvisnosti niso potrebne.

> Opomba: v delovni kopiji je že necommitana sprememba `.github/workflows/ci-cd.yml`, ki ni del te naloge – v commite te funkcionalnosti je ne vključim.

## OpenAPI

`operationId` je v `snake_case` (npr. `consultations_form`, `consultation_schedule_submit`); ogen iz njega naredi ime metode v CamelCase.

```yaml
  /doctors:
    get:
      security:
        - sessionCookie: []
      operationId: doctors_form            # → DoctorsForm
      responses:
        "200":
          $ref: "#/components/responses/200"
        "default":
          $ref: "#/components/responses/default"
  /doctors/{id}/availability:
    post:
      security:
        - sessionCookie: []
      operationId: doctor_availability_submit   # → DoctorAvailabilitySubmit
      parameters:
        - $ref: "#/components/parameters/UUID"
      requestBody:
        required: true
        content:
          application/x-www-form-urlencoded:
            schema:
              type: object
              properties:
                available:
                  type: boolean
              required:
                - available
      responses:
        "303":
          $ref: "#/components/responses/303"
        "default":
          $ref: "#/components/responses/default"
```

- Neveljaven UUID ali `available`, ki ni `true`/`false`, zavrne že ogen (400).
- Neprijavljen uporabnik: `HandleSessionCookie` vrne `errNotLoggedIn`, error handler preusmeri `303 → /auth/login` (AC6) – brez nove kode.

## Avtorizacija (Rego)

Imena operacij za `IsAllowed` (ločena od OpenAPI `operationId`):

| Operacija | Pravilo |
|---|---|
| `doctors_form` | **že obstaja** v `authorization/policies/doctors.rego` (samo `admin`) in je že pokrito v `doctors_test.rego` – ponovno uporabim |
| `change_doctor_availability` | **novo** |

```rego
allow if {
	input.operation_id == "change_doctor_availability"
	input.caller.Role == "admin"
}
```

Target je prazen (`authorization.Target{}`): administrator sme spremeniti vsakega zdravnika, zato lastništvo ni pomembno.

## Handler – `cmd/cybermedico/api/html/doctors.go`

```go
type DoctorsHandler struct {
	authService *AuthService
	db          *pgxpool.Pool
}

func NewDoctorsHandler(authService *AuthService, db *pgxpool.Pool) DoctorsHandler

type DoctorRow struct {
	ID             string
	DisplayName    string
	Specialization string
	Workplace      string
	Available      bool
}
```

### `DoctorsForm(ctx) (api.DoctorsFormRes, error)`

1. `IsAllowed(ctx, "doctors_form", authorization.Target{})` **pred branjem podatkov**; `false` → `herrors.HtmlError{Code: 403, Msg: "You are not allowed to manage doctors", ButtonLink: "/dashboard"}`.
2. SQL (spodaj) → `pgx.CollectRows(rows, pgx.RowToStructByName[DoctorRow])`; napake ovite (`fmt.Errorf("failed to query doctors: %w", err)`).
3. Izris `DoctorsPage(doctors)` v `bytes.Buffer` → `&api.R200{Data: &buff}`.

### `DoctorAvailabilitySubmit(ctx, req, params) (api.DoctorAvailabilitySubmitRes, error)`

1. `IsAllowed(ctx, "change_doctor_availability", authorization.Target{})` **najprej** → 403 ob `false` (AC5). Tako zdravnik ne more niti ugotoviti, ali ID obstaja.
2. `UPDATE` (spodaj); `tag.RowsAffected() == 0` → `herrors.HtmlError{Code: 404, Msg: "Doctor not found", ButtonLink: "/doctors"}` (AC7).
3. `&api.R303{Location: api.NewOptString("/doctors")}` (AC2, AC4).

Transakcija ni potrebna (en sam `UPDATE`). Ni opravil River in ni e-pošte (izven obsega).

### Pomožne funkcije (čiste, testirane)

```go
// availabilityLabel returns "Available" or "Unavailable".
func availabilityLabel(available bool) string
// availabilityAction returns the button text and the value the button sends:
// (true) → ("Mark unavailable", "false"), (false) → ("Mark available", "true").
func availabilityAction(available bool) (label, next string)
```

## SQL

Seznam (AC1 – vsi zdravniki, tudi nerazpoložljivi, urejeni po imenu):

```sql
SELECT id, display_name, specialization, workplace, available
FROM doctor_profile
ORDER BY display_name, id
```

(`id` kot drugi ključ, da je vrstni red stabilen pri enakih imenih.)

Sprememba (AC2, AC4, AC7, AC8):

```sql
UPDATE doctor_profile
SET available = @available
WHERE id = @id
```

z `pgx.NamedArgs{"id": params.ID.String(), "available": request.Available}`. Poizvedba se dotakne samo `doctor_profile`; tabele `consultations`, `written_consultations`, `video_consultations` ostanejo nespremenjene (AC8). Brez lepljenja nizov.

## Stran – `cmd/cybermedico/api/html/doctors.templ`

`templ DoctorsPage(doctors []DoctorRow)` v `@layouts.Dashboard(nil)`, po wireframu `docs/wireframes/05-zdravniki.svg`:

- naslov *Doctors* (komponenta `headinggroup`);
- tabela `class="striped"` (kot `consultations.templ`) s stolpci *Name*, *Specialization*, *Workplace*, *Status*, *Action*;
- status: *Available* / *Unavailable* (`availabilityLabel`), vizualno ločen (npr. `<mark>` ali razred za nerazpoložljive);
- v vsaki vrstici:
  ```html
  <form method="POST" action="/doctors/{id}/availability">
      <input type="hidden" name="available" value="{next}"/>
      <button type="submit" class="outline">{label}</button>
  </form>
  ```
- prazen seznam: *No doctors yet.*

Gumba *Add Doctor* iz wireframa ne dodam (dodajanje zdravnikov je izven obsega).

## Stranska vrstica – `components/sidebar.templ`

- Nova funkcija `util.GetRole(ctx) string` (vzorec `GetUserID`), ki bere `UserRoleContextKey`, nastavljen v `SessionMiddleware`.
- V `Sidebar` med *Overview* in *Content* nov razdelek, kot v wireframu:
  ```templ
  if util.GetRole(ctx) == "admin" {
      <ul>
          <li><span class="nav-section-label">Management</span></li>
          <li><a href="/doctors" class={ util.ClassIfActive(currentPath, "/doctors") } role={ util.RoleIfActive(currentPath, "/doctors") }>…ikona… Doctors</a></li>
      </ul>
  }
  ```
- Skrivanje povezave je samo UI; pravo zaščito zagotavlja Rego v handlerju.

## Testi po sprejemnih kriterijih

Omejitev: Hurl testi se ne morejo prijaviti (prijava zahteva TOTP), Go testi pa po skillu `write-tests` ne smejo uporabljati baze. Zato so prijavljeni tokovi pokriti z Rego testi, Go testi čiste logike/izrisa in ročnim preverjanjem.

| AC | Test | Kje |
|---|---|---|
| **AC1** seznam z imenom, specializacijo, delovnim mestom, statusom | `TestDoctorsPageRendersAllDoctors`: izris `DoctorsPage` z enim razpoložljivim in enim nerazpoložljivim zdravnikom v buffer; preveri, da sta oba imena, specializaciji, delovni mesti, *Available* in *Unavailable* v HTML. Vrstni red je v SQL (`ORDER BY display_name`) – ročno preverjanje. | `doctors_test.go` |
| | Rego: `test_admin_manages_doctors` že pokriva `doctors_form` | `doctors_test.rego` (obstoječ) |
| **AC2** *Mark unavailable* → `false` | `TestAvailabilityAction` (tabelarični): `true` → (`"Mark unavailable"`, `"false"`), `false` → (`"Mark available"`, `"true"`); `TestAvailabilityLabel`. V `TestDoctorsPageRendersAllDoctors` še: razpoložljiv zdravnik ima obrazec z `action="/doctors/{id}/availability"` in `value="false"`. | `doctors_test.go` |
| | Rego: `test_admin_changes_doctor_availability` | `doctors_test.rego` |
| **AC3** nerazpoložljiv ni na `/`, profil/obrazca 404 | Obstoječe obnašanje (US-01); obstoječi `tests/hurl/doctors.hurl` in `specialists.hurl` ostaneta zelena. Celoten tok (admin preklopi → `/` ga ne prikaže → `/specialists/{id}` 404) ročno. | ročno |
| **AC4** *Mark available* vrne zdravnika | Primer `false` v `TestAvailabilityAction`; ročno preverjanje na `/`. | `doctors_test.go`, ročno |
| **AC5** zdravnik → 403 | `test_doctor_cannot_change_doctor_availability` (+ obstoječi `test_doctor_cannot_manage_doctors` za `doctors_form`). `change_doctor_availability` dodam tudi v seznam `doctor_management_operations`, da ga pokrijejo vsi trije obstoječi testi. | `doctors_test.rego` |
| **AC6** neprijavljen → 303 `/auth/login` | Hurl: `GET /doctors` → 303; `POST /doctors/{id}/availability` z `[Form] available: false` → 303, `Location == "/auth/login"`. Rego: `test_anonymous_cannot_change_doctor_availability`. | `tests/hurl/doctors.hurl`, `doctors_test.rego` |
| **AC7** neobstoječ ID → 404 | Logika je `RowsAffected() == 0` → 404; brez baze je ni mogoče testirati enotsko, zato ročno (prijavljen admin, `POST` z naključnim UUID). | ročno |
| **AC8** konzultacije nespremenjene | `UPDATE` se dotakne samo `doctor_profile` (pregled kode); ročno: pri dr. Carter preveriti `/consultations` pred in po preklopu. | ročno |
| sidebar | `TestGetRole` (kontekst z/brez vloge, tabelarično, vzorec `TestGetUserID`). | `util_test.go` |
| Rego splošno | `test_unknown_role_cannot_change_doctor_availability` (`Role: "nurse"`). | `doctors_test.rego` |

### Ročno preverjanje (`mise run develop`, prijava `admin@cybermedico.com` + TOTP)

1. Sidebar ima *Doctors*; `/doctors` prikaže vse 4 zdravnike po abecedi, vsi *Available*.
2. *Mark unavailable* pri dr. Carter → preusmeritev na `/doctors`, status *Unavailable*, gumb *Mark available*.
3. `/` (odjavljen) ne prikaže dr. Carter; `/specialists/22222222-2222-2222-2222-222222222221` in `/written`, `/video` vrnejo 404.
4. `/consultations` dr. Carter nespremenjene.
5. `curl`/brskalnik: `POST /doctors/<naključen UUID>/availability` → 404.
6. *Mark available* → dr. Carter spet na `/`.
7. Prijava `dr.carter@cybermedico.com`: sidebar brez *Doctors*; `/doctors` → 403.

## Vrstni red dela in commiti

Delo na veji `feature/001-doctor-availability`, en logičen korak na commit:

1. `feat: add change_doctor_availability policy` – Rego pravilo + testi.
2. `feat: add admin doctors page with availability toggle` – `openapi.yml` + generirani `ogen/`, `doctors.go`, `doctors.templ`, `doctors_templ.go`, `main.go`, Go testi. (OpenAPI in handler v istem commitu, ker build brez handlerja pade.)
3. `feat: show Doctors link in sidebar for admins` – `util.GetRole`, `sidebar.templ`, `sidebar_templ.go`, test.
4. `test: cover /doctors login redirect in hurl` – `tests/hurl/doctors.hurl`.

Pred vsakim commitom:

```
mise exec -- templ generate        # če se je spremenil *.templ
mise run generate                  # če se je spremenil openapi.yml
mise exec -- golangci-lint run ./...
mise exec -- go test ./...
mise run test:policies
```

in ob zagnani aplikaciji `mise run test:hurl`.

## Tveganja

- **CSRF**: obrazec je navaden `POST` brez žetona, kot obstoječi obrazci (`/consultations/{id}/schedule`); zaščita je odvisna od nastavitev piškotka seje (SameSite). Brez spremembe v tem obsegu.
- **Hkratni zahtevki**: eksplicitna ciljna vrednost poskrbi, da zadnji klik zmaga brez nepričakovanega dvojnega preklopa.
- **Rego ime `doctors_form`** že obstaja iz prejšnjega dela in je deljeno z drugimi bodočimi admin stranmi za zdravnike – namerno ponovno uporabljeno.
