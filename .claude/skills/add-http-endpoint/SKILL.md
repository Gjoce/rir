---
name: add-http-endpoint
description: Uporabi, ko dodajaš novo HTTP pot, stran ali Datastar končno točko v CyberMedico (npr. nova stran za zdravnika ali administratorja, nov obrazec, nov gumb, ki kliče strežnik). Vodi skozi openapi.yml, generiranje ogen, handler, templ stran, Rego pravilo in teste.
---

# Dodajanje nove HTTP končne točke

Vse HTTP poti so opisane v `cmd/cybermedico/openapi.yml`, strežniška koda pa je generirana z ogen. Ročno nikoli ne urejaj `cmd/cybermedico/ogen/`.

## Koraki

1. **Preberi kontekst**
   - Uporabniško zgodbo ali specifikacijo (`docs/05-uporabniske-zgodbe.md`, `docs/specs/NNN-*/spec.md`).
   - Podoben obstoječ handler v `cmd/cybermedico/api/html/` (npr. `dashboard.go`, `consultations.go`) in uporabi isti vzorec.

2. **Dodaj pot v `cmd/cybermedico/openapi.yml`**
   - Unikaten `operationId` v snake_case kot obstoječe poti (npr. `doctors_form`; ogen iz njega naredi ime metode `DoctorsForm`).
   - Stran za prijavljene uporabnike: `security: - sessionCookie: []`.
   - Odgovori: uporabi obstoječe `$ref: "#/components/responses/200"` in `"default"`.
   - Parameter ID: `$ref: "#/components/parameters/UUID"`.

3. **Generiraj strežnik**: `mise run generate`, nato `mise exec -- go build ./...`. Build pade, dokler handler ne obstaja, kar je pričakovano.

4. **Napiši handler** v `cmd/cybermedico/api/html/<področje>.go`
   - Metoda na obstoječem ali novem `XxxHandler` struct, podpis iz generiranega vmesnika v `cmd/cybermedico/ogen/oas_server_gen.go`.
   - Request in response dobiš z `util.RequestFromContext(ctx)` / `util.ResponseFromContext(ctx)`.
   - **Avtorizacija pred vsem drugim**: `h.authService.IsAllowed(ctx, "<operacija>", authorization.Target{...})`; ob `false` vrni `herrors.HtmlError{Code: http.StatusForbidden, ...}`.
   - SQL samo parametriziran (`$1` ali `@ime` s `pgx.NamedArgs`), napake ovij z `fmt.Errorf("...: %w", err)`.
   - Uporabniku prikazne napake vrni kot `herrors.HtmlError` (Code, Msg, ButtonLink).
   - Nov handler struct dodaj v `initializeHTMLServer` v `cmd/cybermedico/main.go` (to je sprememba `main.go`, omeni jo v PR).

5. **Stran ali del strani** (če je potreben)
   - `*.templ` ob handlerju (stran) ali v `components/` (ponovno uporabna komponenta), postavitev iz `layouts/`.
   - Besedila v UI so v angleščini.
   - `mise exec -- templ generate`, generirane `*_templ.go` commitaj skupaj z `.templ`.

6. **Avtorizacijsko pravilo** v `authorization/policies/<področje>.rego`
   - `allow if { input.operation_id == "<operacija>"; ... }` za vsako vlogo posebej.
   - Testi v `authorization/policies_test/<področje>_test.rego` (glej skill `write-tests`): dovoljeno za pravo vlogo, zavrnjeno za anonimnega in napačno vlogo.

7. **Testi**: Go enotski testi za čisto logiko (skill `write-tests`) in Hurl test v `tests/hurl/` za odziv brez prijave (npr. `303` na `/auth/login`).

8. **Preveri pred oddajo** (iz AGENTS.md):
   ```
   mise exec -- golangci-lint run ./...
   mise exec -- go test ./...
   mise run test:policies
   ```
   Če teče aplikacija: `mise run test:hurl`.

## Kontrolni seznam

- [ ] pot in `operationId` v `openapi.yml`, `mise run generate` pognan
- [ ] handler preveri avtorizacijo, preden bere podatke
- [ ] SQL parametriziran, napake ovite
- [ ] templ generiran, `*_templ.go` dodan
- [ ] Rego pravilo + Rego testi
- [ ] Go in/ali Hurl testi
- [ ] lint, Go testi in Rego testi zeleni
