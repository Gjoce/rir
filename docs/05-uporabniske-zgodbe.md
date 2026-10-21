# Uporabniške zgodbe

Zgodbe pokrivajo vseh 11 funkcionalnosti iz [02-funkcionalnosti-in-obseg.md](02-funkcionalnosti-in-obseg.md) (F1-F11). Funkcionalnosti F1-F7 so MVP.

| Oznaka | Funkcionalnost                           | Vloga         | MVP |
| ------ | ---------------------------------------- | ------------- | --- |
| US-01  | F1 Naročilo pisne konzultacije           | Pacient       | ✔   |
| US-02  | F1 Naročilo pisne konzultacije           | Pacient       | ✔   |
| US-03  | F2 Naročilo video konzultacije           | Pacient       | ✔   |
| US-04  | F3 Pisni odgovor zdravnika               | Zdravnik      | ✔   |
| US-05  | F4 Določitev termina video klica         | Zdravnik      | ✔   |
| US-06  | F4 Določitev termina video klica         | Pacient       | ✔   |
| US-07  | F5 Video klic v brskalniku               | Pacient       | ✔   |
| US-08  | F5 Video klic v brskalniku               | Zdravnik      | ✔   |
| US-09  | F6 Digitalno podpisano mnenje            | Pacient       | ✔   |
| US-10  | F7 Samodejni opomniki                    | Zdravnik      | ✔   |
| US-11  | F7 Označevanje neopravljenih konzultacij | Administrator | ✔   |
| US-12  | F8 Profili zdravnikov                    | Administrator |     |
| US-13  | F9 Upravljanje provizij                  | Administrator |     |
| US-14  | F10 Nadzorna plošča s statistiko         | Zdravnik      |     |
| US-15  | F11 Dvostopenjska prijava                | Zdravnik      |     |

---

### US-01 - Izbira specialista

**Funkcionalnost:** F1 Naročilo pisne konzultacije · **Vloga:** Pacient

Kot **pacient** želim **pregledati razpoložljive specialiste z njihovo specializacijo, jeziki in ceno**, da **izberem zdravnika, ki ustreza moji težavi in proračunu**.

**Acceptance criteria:**

- Seznam prikaže le zdravnike z oznako `available`; vsak prikazuje ime, fotografijo, specializacijo, delovno mesto, jezike ter ceno pisne in video konzultacije.
- Seznam je mogoče filtrirati po specializaciji; po izbiri filtra so prikazani le zdravniki s to specializacijo.
- Klik na zdravnika odpre njegov profil s kratko predstavitvijo (bio) in gumboma "Pisna konzultacija" in "Video konzultacija".
- Če ni na voljo nobenega zdravnika izbrane specializacije, se prikaže sporočilo "Trenutno ni razpoložljivih zdravnikov" in povezava na celoten seznam.

### US-02 - Oddaja pisne konzultacije

**Funkcionalnost:** F1 Naročilo pisne konzultacije · **Vloga:** Pacient

Kot **pacient** želim **opisati svojo težavo in priložiti izvide**, da **mi zdravnik lahko poda pisno mnenje brez obiska ambulante**.

**Acceptance criteria:**

- Obvezna polja so ime, priimek, starost, e-pošta, telefon in opis težave; neobvezna so diagnoza, zdravila, alergije in oznaka "potrebujem prevajalca". Brez obveznih polj obrazca ni mogoče oddati, ob praznem polju se izpiše napaka.
- Priložiti je mogoče največ **5 datotek** tipa PDF, JPG ali PNG, vsaka do **10 MB**; pri prekoračitvi se izpiše napaka z navedeno omejitvijo, obrazec pa ohrani vnesene podatke.
- Oddaja ni mogoča brez potrditve pogojev uporabe.
- Po oddaji se ustvari konzultacija s statusom `pending` in rokom za odgovor 72 ur, pacient pa prejme potrditveno e-pošto z izbranim zdravnikom in rokom.

### US-03 - Naročilo video konzultacije

**Funkcionalnost:** F2 Naročilo video konzultacije · **Vloga:** Pacient

Kot **pacient** želim **naročiti video konzultacijo in izbrati, kateri del dneva mi ustreza**, da **se z zdravnikom pogovorim v času, ko sem na voljo**.

**Acceptance criteria:**

- Obrazec vsebuje ista polja kot pisna konzultacija (US-02) in dodatno obvezno izbiro dela dneva: dopoldne, popoldne ali zvečer.
- Pacient lahko konzultacijo označi kot **nujno**; nujna konzultacija ima rok za določitev termina **8 ur**, nenujna **12 ur** od oddaje.
- Prikazana cena je cena video konzultacije izbranega zdravnika.
- Po oddaji se ustvari konzultacija s statusom `pending`, pacient prejme potrditveno e-pošto z navedbo, da bo termin prejel naknadno.

### US-04 - Pisni odgovor zdravnika

**Funkcionalnost:** F3 Pisni odgovor zdravnika · **Vloga:** Zdravnik

Kot **zdravnik** želim **prebrati podatke pacienta in priloge ter napisati odgovor v urejevalniku**, da **pacientu podam strokovno mnenje v dogovorjenem roku**.

**Acceptance criteria:**

- Zdravnik vidi le konzultacije, ki so mu dodeljene; dostop do tuje konzultacije vrne napako 403.
- Stran konzultacije prikazuje vse podatke iz obrazca, seznam prilog (možen prenos) in preostali čas do roka (odštevanje).
- Odgovor je mogoče oddati le pri pisni konzultaciji v statusu `pending`; prazen odgovor ni dovoljen.
- Pred oddajo mora zdravnik vnesti veljavno TOTP kodo, tudi če je že prijavljen. Ker se odgovor digitalno podpiše z zdravnikovim ključem, ponovna potrditev zagotavlja, da ga oddaja res zdravnik in ne nekdo, ki je dostopil do njegove odprte seje. Ob napačni kodi odgovor ni oddan, vneseno besedilo pa ostane.
- Po oddaji se status spremeni v `finished`, nastavi se `finished_at`, urejanje odgovora pa ni več mogoče.

### US-05 - Določitev termina video klica

**Funkcionalnost:** F4 Določitev termina video klica · **Vloga:** Zdravnik

Kot **zdravnik** želim **določiti točen termin video klica glede na pacientov želeni del dneva**, da **se klic izvede v času, ki ustreza obema**.

**Acceptance criteria:**

- Gumb "Določi termin" je na voljo le pri video konzultaciji v statusu `pending`, ki pripada prijavljenemu zdravniku.
- Pri konzultaciji sta prikazana pacientov želeni del dneva in preostali čas do roka za določitev (12 h oz. 8 h pri nujni).
- Termina v preteklosti ni mogoče izbrati; sistem izpiše napako.
- Po potrditvi se status spremeni v `scheduled`, rok za mnenje po klicu pa se nastavi na termin + 24 ur.

### US-06 - Obvestilo o terminu

**Funkcionalnost:** F4 Določitev termina video klica · **Vloga:** Pacient

Kot **pacient** želim **po e-pošti prejeti termin in povezavo za video klic**, da **vem, kdaj in kako se pridružim, brez ustvarjanja računa**.

**Acceptance criteria:**

- E-pošta je poslana takoj, ko zdravnik določi termin, in vsebuje ime zdravnika, datum in uro (oblika `DD.MM.LLLL HH:MM`) ter osebno povezavo za vstop.
- Pacient prejme opomnik z isto povezavo **1 uro pred** terminom.
- Povezava velja samo za to konzultacijo; s spremenjenim ID-jem konzultacije v povezavi vstop ni mogoč.

### US-07 - Vstop pacienta v video klic

**Funkcionalnost:** F5 Video klic v brskalniku · **Vloga:** Pacient

Kot **pacient** želim **se video klicu pridružiti s klikom na povezavo v brskalniku**, da **mi ni treba nameščati nobene aplikacije**.

**Acceptance criteria:**

- Povezava omogoča vstop od **10 minut pred** do **60 minut po** začetku termina; zunaj tega okna se prikaže sporočilo z datumom in uro termina.
- Brskalnik pred vstopom zahteva dovoljenje za kamero in mikrofon; če ga pacient zavrne, se prikaže navodilo, kako dovoljenje omogočiti.
- Pacient vidi lastno sliko in sliko zdravnika ter ima gumbe za izklop mikrofona, izklop kamere in zapustitev klica.
- Če neposredna povezava ni mogoča, se klic samodejno vzpostavi prek TURN strežnika, brez dejanja pacienta.

### US-08 - Izvedba klica in mnenje po klicu

**Funkcionalnost:** F5 Video klic v brskalniku · **Vloga:** Zdravnik

Kot **zdravnik** želim **iz nadzorne plošče vstopiti v video klic in po njem zapisati mnenje**, da **pacientu po pogovoru podam tudi pisna priporočila**.

**Acceptance criteria:**

- Gumb "Pridruži se klicu" je viden le pri konzultacijah v statusu `scheduled`, dodeljenih prijavljenemu zdravniku.
- Po klicu lahko zdravnik odda pisno mnenje najpozneje **24 ur** po terminu; po oddaji se status spremeni v `finished`.
- V klicu sta lahko največ dva udeleženca; tretja povezava v isto sobo je zavrnjena.

### US-09 - Prejem podpisanega mnenja

**Funkcionalnost:** F6 Digitalno podpisano mnenje · **Vloga:** Pacient

Kot **pacient** želim **prejeti digitalno podpisano mnenje zdravnika po e-pošti**, da **ga lahko predložim drugemu zdravniku kot verodostojno**.

**Acceptance criteria:**

- Po oddaji odgovora (US-04) ali mnenja po klicu (US-08) pacient prejme e-pošto, ki vsebuje celotno mnenje zdravnika, ime zdravnika in datum oddaje.
- Mnenje je podpisano s ključem Ed25519 zdravnika, ki ga je oddal; podpis zajema vse podatke konzultacije in odgovor.
- Sprememba katerega koli podpisanega polja po podpisu povzroči neuspešno preverjanje podpisa.
- E-pošta je poslana natanko enkrat (`email_sent_patient = true`); ponovna obdelava opravila ne pošlje druge kopije.

### US-10 - Opomnik pred potekom roka

**Funkcionalnost:** F7 Samodejni opomniki · **Vloga:** Zdravnik

Kot **zdravnik** želim **prejeti e-poštni opomnik, preden poteče rok za konzultacijo**, da **ne zamudim odgovora pacientu**.

**Acceptance criteria:**

- Za pisno konzultacijo brez odgovora zdravnik prejme opomnik **12 ur** pred potekom 72-urnega roka.
- Za video konzultacijo brez termina zdravnik prejme opomnik pred potekom roka za določitev termina; za mnenje po klicu prejme opomnik **3 ure** pred potekom 24-urnega roka.
- Opomnik vsebuje ime pacienta, vrsto konzultacije, rok in povezavo na konzultacijo.
- Za isti rok je poslan največ en opomnik; če zdravnik pred tem odgovori, opomnik ni poslan.

### US-11 - Pregled neopravljenih konzultacij

**Funkcionalnost:** F7 Označevanje neopravljenih konzultacij · **Vloga:** Administrator

Kot **administrator** želim **videti konzultacije, ki jih zdravnik ni opravil v roku**, da **lahko pacientu ponudim drugega zdravnika ali vračilo**.

**Acceptance criteria:**

- Ko rok poteče brez odgovora ali termina, sistem konzultaciji samodejno nastavi status `not_attended`.
- Administrator vidi seznam vseh konzultacij s statusom `not_attended` s pacientom, zdravnikom, vrsto in časom poteka roka.
- Zdravnik konzultacije s statusom `not_attended` ne vidi več in nanjo ne more odgovoriti.

### US-12 - Dodajanje zdravnika

**Funkcionalnost:** F8 Profili zdravnikov · **Vloga:** Administrator

Kot **administrator** želim **dodati zdravnika s profilom, cenami in jeziki**, da **ga pacienti lahko izberejo za konzultacijo**.

**Acceptance criteria:**

- Obvezna polja so e-pošta, ime za prikaz, specializacija, delovno mesto, fotografija, predstavitev, ceni pisne in video konzultacije, provizija in bančni račun; vsaj en jezik je obvezen.
- E-pošta mora biti edinstvena; ob podvojeni e-pošti se izpiše napaka.
- Po shranjevanju zdravnik prejme e-pošto z dostopnimi podatki, ob prvi prijavi pa mora nastaviti dvostopenjsko prijavo (US-15).
- Administrator lahko zdravnika označi kot nerazpoložljivega; takšen zdravnik se ne prikaže v seznamu za paciente (US-01).

### US-13 - Sprememba provizije

**Funkcionalnost:** F9 Upravljanje provizij · **Vloga:** Administrator

Kot **administrator** želim **spremeniti provizijo zdravnika z obrazložitvijo**, da **je zdravnik obveščen o spremembi in razlogu zanjo**.

**Acceptance criteria:**

- Provizija je celo število med 0 in 100 (odstotki); druge vrednosti sistem zavrne.
- Obrazložitev spremembe je obvezna.
- Po shranjevanju zdravnik prejme e-pošto z novo provizijo in obrazložitvijo.
- Provizijo lahko spremeni le administrator; zdravnik na svojem profilu provizijo vidi, ne more pa je urejati.

### US-14 - Statistika na nadzorni plošči

**Funkcionalnost:** F10 Nadzorna plošča s statistiko · **Vloga:** Zdravnik

Kot **zdravnik** želim **na nadzorni plošči videti število svojih konzultacij in prihodek v tekočem mesecu**, da **spremljam svojo obremenitev in zaslužek**.

**Acceptance criteria:**

- Nadzorna plošča prikazuje prihodek tekočega meseca (le plačane konzultacije) v primerjavi s prejšnjim mesecem.
- Prikazan je graf števila konzultacij po vrsti (pisna / video) in po statusu.
- Zdravnik vidi le podatke svojih konzultacij; administrator na isti strani vidi podatke vseh zdravnikov.

### US-15 - Dvostopenjska prijava

**Funkcionalnost:** F11 Dvostopenjska prijava · **Vloga:** Zdravnik

Kot **zdravnik** želim **se prijaviti z geslom in kodo iz aplikacije za preverjanje pristnosti**, da **do zdravstvenih podatkov pacientov ne more dostopati nihče, ki pozna le moje geslo**.

**Acceptance criteria:**

- Ob prvi prijavi sistem prikaže QR kodo za aplikacijo za preverjanje pristnosti (npr. Google Authenticator); do nadzorne plošče ni dostopa, dokler zdravnik ne potrdi prve kode.
- Ob vsaki naslednji prijavi je po pravilnem geslu obvezna 6-mestna koda; sprejete so kode trenutnega 30-sekundnega okna ± 1 okno.
- Ista koda ne more biti uporabljena dvakrat.
- TOTP koda se iz varnostnih razlogov zahteva tudi pri oddaji pisnega odgovora (US-04), saj je to dejanje, ki ga sistem digitalno podpiše v imenu zdravnika.
- Ob napačnem geslu ali kodi se izpiše splošno sporočilo "Napačni prijavni podatki", ki ne razkrije, kateri podatek je napačen.
