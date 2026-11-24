# Pregled prevzetega projekta

**Projekt:** CyberMedico (izvorni repozitorij [Gjoce/rir](https://github.com/Gjoce/rir), fork [SimeonJovanovski32/rir](https://github.com/SimeonJovanovski32/rir))
**Pregledal:** Simeon Jovanovski · 24. 11. 2026

CyberMedico je platforma za spletne konzultacije s specialisti. Pacient brez registracije naroči pisno ali video konzultacijo, zdravnik odgovori v določenem roku, mnenje pa je digitalno podpisano in poslano po e-pošti.

## Pregled projekta

- **Arhitektura:** monolit v Go (ADR 0001) s strežniškim izrisovanjem prek templ in Datastar (ADR 0002). HTTP vmesnik je opisan v `openapi.yml`, koda strežnika pa je generirana z ogen (ADR 0003). Podatki so v PostgreSQL, priloge in podpisi v S3 združljivi shrambi, opravila v ozadju (e-pošta, opomniki, podpisovanje) pa tečejo prek River.
- **Varnost:** prijava zdravnikov z geslom in obvezno TOTP kodo, avtorizacija s pravili Open Policy Agent (Rego), povezave za paciente s HMAC žetoni, mnenja podpisana z Ed25519.
- **Dokumentacija:** zelo dobra, vsebuje zasnovo, obseg, podatkovni model, 12 arhitekturnih odločitev (ADR), uporabniške zgodbe, wireframe in poročilo o preverjanju MVP.
- **Testi ob prevzemu:** 51 Go testov (video signalizacija, žetoni, podpisi), 49 Rego testov za pravila avtorizacije in 12 datotek Hurl HTTP testov, ki potrebujejo zagnano aplikacijo.
- **Lokalni zagon:** `mise install` in `mise run develop` po navodilih iz README. Aplikacija se je zagnala brez napak (migracije, začetni podatki, ključi zdravnikov).

## Implementirane funkcionalnosti (MVP)

Vseh sedem MVP funkcionalnosti iz `02-funkcionalnosti-in-obseg.md` je implementiranih:

| | Funkcionalnost | Stanje |
|---|---|---|
| F1 | Naročilo pisne konzultacije s prilogami | ✅ |
| F2 | Naročilo video konzultacije z delom dneva in nujnostjo | ✅ |
| F3 | Pisni odgovor zdravnika v 72 urah | ✅ |
| F4 | Določitev termina video klica (12 h, nujno 8 h) | ✅ |
| F5 | Video klic v brskalniku (WebRTC, TURN) | ✅ |
| F6 | Digitalno podpisano mnenje | ✅ |
| F7 | Samodejni opomniki in označevanje neopravljenih konzultacij | ✅ |

Poleg MVP sta izvedeni tudi nadzorna plošča s statistiko (F10) in dvostopenjska prijava (F11). Profili zdravnikov in upravljanje provizij (F8, F9) niso del MVP in niso dokončani.

## Ujemanje z uporabniškimi zgodbami

Pregledanih je bilo 5 uporabniških zgodb iz `05-uporabniske-zgodbe.md`.

| Zgodba | Opis | Ali zadošča? | Opomba |
|---|---|---|---|
| **US-01** Izbira specialista | Seznam razpoložljivih zdravnikov, filter po specializaciji, profil | ✅ Da | Prikazani so le zdravniki z `available = true`, nerazpoložljiv vrne 404. Sporočilo, ko ni zdravnikov, je v angleščini ("No doctors are currently available."). |
| **US-02** Oddaja pisne konzultacije | Obvezna polja, do 5 prilog po 10 MB, pogoji uporabe, rok 72 ur | ✅ Da | Omejitve so v kodi (`maxAttachmentCount = 5`, `maxAttachmentSize = 10 MB`), rok 72 ur nastavi sprožilec v bazi, potrditvena e-pošta prispe v Mailpit. |
| **US-04** Pisni odgovor zdravnika | Le lastne konzultacije, priloge, odštevanje, TOTP pred oddajo | ≈ Večinoma | Vse zahteve so izpolnjene, tuja konzultacija vrne 403. Priloge iz začetnih podatkov nimajo datotek v shrambi, zato njihov prenos ne deluje (priloge novih naročil delujejo). |
| **US-05** Določitev termina | Termin le za lastno video konzultacijo, ne v preteklosti, rok za mnenje +24 h | ✅ Da | Termin v preteklosti je zavrnjen, status preide v `scheduled`, rok za mnenje nastavi sprožilec. |
| **US-09** Digitalno podpisano mnenje | Podpis Ed25519 vseh podatkov, e-pošta natanko enkrat | ✅ Da | Pokrito z Go testi (`signing`); sprememba kateregakoli podpisanega polja povzroči neuspešno preverjanje. |

**Zaključek:** implementirane funkcionalnosti zadoščajo pregledanim uporabniškim zgodbam. Razlike so manjše in dokumentirane v `07-preverjanje-mvp.md` (angleški vmesnik, priloge v začetnih podatkih).

## Ugotovitve in izboljšave ob prevzemu

- Projekt ni imel CI/CD cevovoda, konfiguracije linterja v repozitoriju (golangci-lint je bil zagnan s privzetimi nastavitvami) ne Dockerfila za namestitev. Vse to je dodano v drugem delu naloge.
- Datoteka `cmd/cybermedico/api/html/auth.go` ni bila formatirana po `gofmt` (vrstni red uvozov); popravljeno.
- Paket `cmd/cybermedico/api/util` ni imel testov; dodanih je 16 enotskih testov (pokritost paketa 95,8 %).
- Začetni podatki (`compose.sql`) ob vsakem zagonu izpraznijo tabele. To je primerno za razvoj in predstavitev, v pravi produkciji pa mora biti `CYBERMEDICO_SEED_FILE` prazen.
