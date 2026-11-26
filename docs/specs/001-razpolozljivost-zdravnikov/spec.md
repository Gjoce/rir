# 001 – Razpoložljivost zdravnikov

**Uporabniška zgodba:** US-12 (F8 Profili zdravnikov), zadnji sprejemni kriterij
**Vloga:** Administrator
**Status:** specifikacija

## Uporabniška zgodba

Kot **administrator** želim **videti seznam vseh zdravnikov in posameznega zdravnika označiti kot razpoložljivega ali nerazpoložljivega**, da **pacienti ne morejo naročiti konzultacije pri zdravniku, ki trenutno ne sprejema novih konzultacij** (npr. dopust, bolniška, preobremenjenost).

## Ozadje

- Tabela `doctor_profile` že ima stolpec `available BOOLEAN NOT NULL DEFAULT TRUE` (migracija `00006`).
- Javni seznam zdravnikov (`/`) in profil ter obrazec za naročilo (`/specialists/{id}`, `/specialists/{id}/written`, `/specialists/{id}/video`) že prikazujejo samo zdravnike z `available = true` (US-01).
- Manjka stran, kjer bi administrator to vrednost videl in spremenil. Izgled: `docs/wireframes/05-zdravniki.svg`.

## Zahteve

1. Nova stran **`/doctors`** za prijavljenega administratorja s seznamom vseh zdravnikov.
2. Za vsakega zdravnika so prikazani: ime za prikaz, specializacija, delovno mesto in trenutna razpoložljivost (*Available* / *Unavailable*).
3. Ob vsakem zdravniku je gumb, ki razpoložljivost preklopi (*Mark unavailable* / *Mark available*).
4. V stranski vrstici (sidebar) je za administratorja povezava **Doctors**.
5. Sprememba se shrani v `doctor_profile.available` in takoj vpliva na javni seznam.

## Sprejemni kriteriji

- **AC1** Prijavljen administrator na `/doctors` vidi vse zdravnike (tudi nerazpoložljive), urejene po imenu, z imenom, specializacijo, delovnim mestom in statusom.
- **AC2** Klik na *Mark unavailable* nastavi `available = false`; po osvežitvi seznama je status *Unavailable*, gumb pa *Mark available*.
- **AC3** Nerazpoložljiv zdravnik se ne prikaže na javnem seznamu `/`, njegov profil in obrazca za naročilo pa vrneta 404 (obstoječe obnašanje iz US-01).
- **AC4** Klik na *Mark available* zdravnika vrne na javni seznam.
- **AC5** Zdravnik (vloga `doctor`) do `/doctors` in do preklopa nima dostopa – vrne se 403.
- **AC6** Neprijavljen uporabnik je ob obisku `/doctors` ali preklopa preusmerjen na `/auth/login` (303).
- **AC7** Preklop za neobstoječ ID zdravnika vrne 404.
- **AC8** Obstoječe konzultacije zdravnika se ob spremembi razpoložljivosti ne spremenijo.

## Izven obsega

- Dodajanje, urejanje in brisanje zdravnikov (preostanek US-12).
- Sprememba provizije (US-13).
- Obvestilo zdravniku po e-pošti ob spremembi razpoložljivosti.
- Načrtovana odsotnost (od–do).

## Tehnične omejitve

- **Brez nove migracije** – uporabi obstoječi stolpec `available`.
- Poti najprej v `cmd/cybermedico/openapi.yml`, nato `mise run generate` (skill `add-http-endpoint`).
- Avtorizacija z Rego pravilom, samo vloga `admin`; testi v `authorization/policies_test/` (skill `write-tests`).
- Besedila v UI v angleščini.
- Lint, Go testi in Rego testi morajo biti zeleni; cevovod CI/CD ostane zelen.
