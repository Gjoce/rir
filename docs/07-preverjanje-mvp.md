# Preverjanje MVP

Preverjanje je bilo izvedeno 11. 11. 2026 na čisti namestitvi: nov klon repozitorija, nova Docker Compose okolja (prazne baze in shrambe) in koraki iz [README](../README.md). Aplikacija se je zagnala brez napak (migracije, začetni podatki, ključi zdravnikov, vedra v shrambi), vsi HTTP testi (`mise run test:hurl`, 12 datotek) in Rego testi (`mise run test:policies`) so uspešni.

Spodaj je za vsako MVP uporabniško zgodbo ([05-uporabniske-zgodbe.md](05-uporabniske-zgodbe.md)) navedeno, kako je bil kriterij preverjen in kje se izvedba razlikuje.

Oznake: ✔ izpolnjeno · ≈ izpolnjeno z razliko (opisana v razdelku Razlike)

| Zgodba | Kriterij | Stanje | Preverjeno |
| --- | --- | --- | --- |
| US-01 | Seznam prikaže le razpoložljive zdravnike z imenom, fotografijo, specializacijo, delovnim mestom, jeziki in cenama | ✔ | Hurl `doctors.hurl`; zdravnik z `available = false` izgine s seznama, njegov profil in obrazec vrneta 404 |
| US-01 | Filter po specializaciji | ✔ | Hurl `doctors.hurl` |
| US-01 | Profil z bio in gumboma za pisno in video konzultacijo | ✔ | Ročno (HTTP) |
| US-01 | Sporočilo in povezava, ko ni zdravnikov izbrane specializacije | ✔ | Hurl `doctors.hurl` (besedilo v angleščini, glej Razlike) |
| US-02 | Obvezna in neobvezna polja, napake ob praznih poljih | ✔ | Hurl `orders.hurl` |
| US-02 | Največ 5 datotek PDF/JPG/PNG do 10 MB, obrazec ohrani podatke | ✔ | Hurl `attachments.hurl`; datoteka 11 MB zavrnjena na strežniku, konzultacija ni ustvarjena |
| US-02 | Brez potrditve pogojev oddaja ni mogoča | ✔ | Hurl `orders.hurl` |
| US-02 | Status `pending`, rok 72 ur, potrditvena e-pošta z zdravnikom in rokom | ✔ | Baza (rok točno 72 h), Mailpit |
| US-03 | Izbira dela dneva, nujnost, cena video konzultacije | ✔ | Hurl `video_orders.hurl` |
| US-03 | Rok za termin 8 ur (nujno) oz. 12 ur | ✔ | Baza (točno 8 h in 12 h) |
| US-03 | Potrditvena e-pošta, da bo termin poslan naknadno | ✔ | Mailpit |
| US-04 | Zdravnik vidi le svoje konzultacije, tuja vrne 403 | ✔ | Brskalnik (403 na konzultaciji drugega zdravnika) |
| US-04 | Vsi podatki, priloge in odštevanje do roka | ≈ | Brskalnik; priloge začetnih podatkov nimajo datotek (glej Razlike) |
| US-04 | Odgovor le pri pisni konzultaciji `pending`, prazen ni dovoljen | ✔ | Pravila Rego, ročno |
| US-04 | TOTP pred oddajo; ob napačni kodi besedilo ostane | ✔ | Brskalnik: "Invalid code. Try again.", besedilo ostane v urejevalniku |
| US-04 | Po oddaji `finished`, `finished_at`, urejanje ni več mogoče | ✔ | Baza, pravila Rego |
| US-05 | Gumb za termin le pri lastni video konzultaciji `pending` | ✔ | Ročno, pravila Rego |
| US-05 | Prikaz želenega dela dneva in odštevanja | ✔ | Brskalnik |
| US-05 | Termin v preteklosti ni mogoč | ✔ | Brskalnik: "Scheduled time needs to be in the future" |
| US-05 | Status `scheduled`, rok za mnenje termin + 24 ur | ✔ | Baza (sproži sprožilec) |
| US-06 | E-pošta z zdravnikom, datumom `DD.MM.LLLL HH:MM` in osebno povezavo | ✔ | Mailpit |
| US-06 | Opomnik 1 uro pred terminom | ✔ | Opravilo River, načrtovano 1 uro pred terminom |
| US-06 | Povezava velja le za to konzultacijo | ✔ | Go testi žetonov, ročno (403 s tujim ID) |
| US-07 | Vstop od 10 minut pred do 60 minut po začetku, sicer sporočilo s terminom | ✔ | Go testi, brskalnik (stran počaka in sama odpre klic) |
| US-07 | Navodilo ob zavrnjeni kameri ali mikrofonu | ✔ | Brskalnik |
| US-07 | Lastna slika, slika zdravnika, gumbi za mikrofon, kamero in izhod | ✔ | Brskalnik (dva zavihka) |
| US-07 | Samodejno prek TURN, ko neposredna povezava ni mogoča | ✔ | Brskalnik s prisiljeno potjo prek lokalnega coturn |
| US-08 | Gumb za vstop le pri lastni konzultaciji `scheduled` | ✔ | Brskalnik, pravila Rego |
| US-08 | Mnenje najpozneje 24 ur po terminu, nato `finished` | ✔ | Brskalnik, Go test meje |
| US-08 | V klicu največ dva udeleženca; tretja povezava je zavrnjena | ≈ | Go test (glej Razlike) |
| US-09 | E-pošta s celotnim mnenjem, zdravnikom in datumom oddaje | ✔ | Mailpit |
| US-09 | Podpis Ed25519 zdravnika zajema vse podatke konzultacije in odgovor | ✔ | Go testi, brskalnik ("signature is valid") |
| US-09 | Sprememba podpisanega polja povzroči neuspešno preverjanje | ✔ | Go test za vsako polje; brskalnik po spremembi v bazi |
| US-09 | E-pošta natanko enkrat | ✔ | Ponovljeno opravilo ne pošlje druge kopije |
| US-10 | Opomniki 12 h / pred rokom za termin (3 h) / 3 h pred rokom za mnenje | ✔ | Opravila River, Mailpit |
| US-10 | Opomnik s pacientom, vrsto, rokom in povezavo | ✔ | Mailpit |
| US-10 | Največ en opomnik na rok, nobenega po odgovoru | ✔ | Podvojeno opravilo in odgovorjena konzultacija ne pošljeta e-pošte |
| US-11 | Samodejni status `not_attended` ob poteku roka | ✔ | Opravila River za vse tri roke |
| US-11 | Seznam za administratorja s pacientom, zdravnikom, vrsto in časom poteka roka | ✔ | Brskalnik (zavihek Not Attended) |
| US-11 | Zdravnik takih konzultacij ne vidi in nanje ne more odgovoriti | ✔ | Brskalnik (403) |

## Razlike

- **Jezik vmesnika (US-01 in drugi).** Aplikacija in e-pošta sta v angleščini, zato so sporočila v kriterijih (npr. "Trenutno ni razpoložljivih zdravnikov") prikazana v angleškem prevodu ("No doctors are currently available.").
- **Tretja povezava v klic (US-08).** V sobi sta vedno največ dva udeleženca, en zdravnik in en pacient. Nova povezava iste strani (npr. drug zavihek ali ponovna povezava po izpadu omrežja) prevzame mesto stare, ki se zapre s kodo 4001 in se ne poveže znova; stran sporoči "The call was opened in another window". Zavrnitev bi onemogočila vrnitev v klic po izpadu, dokler se stara povezava ne izteče, zato je izbrano prevzemanje.
- **Priloge v začetnih podatkih (US-04).** Začetni podatki (`compose.sql`) vsebujejo zapise prilog, ne pa tudi datotek v shrambi, zato prenos teh prilog vrne napako shrambe. Priloge konzultacij, oddanih prek aplikacije, se shranijo in prenesejo normalno.
- **Opomnik za termin (US-10).** Kriterij ne določa časa; opomnik je poslan 3 ure pred rokom za določitev termina.

## Izven MVP

US-12 in US-13 (profili zdravnikov, provizije) niso del MVP. Nadzorna plošča s statistiko (US-14) je izvedena po vzoru cybermed-go: grafi so definirani v tabeli `dashboard_widgets` po vlogah in se naložijo prek `/widgets/{id}`; zdravnik vidi le svoje podatke, pripomočkov druge vloge ni mogoče naložiti (404). Dvostopenjska prijava (US-15) je izvedena, ker jo zahteva oddaja odgovora (US-04).
