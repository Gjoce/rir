# Funkcionalnosti in obseg

## Uporabniške vloge

| Vloga | Kaj lahko počne | Česa ne more |
|---|---|---|
| **Pacient** (brez računa) | Naroči pisno ali video konzultacijo, priloži izvide, se prek povezave iz e-pošte pridruži video klicu, prejme podpisano mnenje. | Nima prijave ne nadzorne plošče, ne vidi drugih konzultacij, oddane konzultacije ne more spreminjati, video klicu se lahko pridruži le v času termina. |
| **Zdravnik** | Vidi svoje konzultacije, odda pisni odgovor, določi termin in izvede video klic, odda mnenje po klicu, vidi svojo statistiko. | Ne vidi konzultacij drugih zdravnikov, ne spreminja cen ali svoje provizije, ne dodaja zdravnikov, ne dostopa do konzultacij, ki so mu potekle. |
| **Administrator** | Dodaja, ureja in briše zdravnike, določa provizije, vnaša konzultacije, vidi vse konzultacije (tudi neopravljene) in celotno statistiko. | Ne odgovarja na konzultacije in se ne pridruži video klicu - to lahko stori le dodeljeni zdravnik. |

## Predvidene funkcionalnosti

1. **[MVP]** Naročilo **pisne konzultacije** z opisom težave, diagnozo, zdravili, alergijami in prilogami (izvidi).
2. **[MVP]** Naročilo **video konzultacije** z izbiro želenega dela dneva (dopoldne / popoldne / zvečer) in oznako nujnosti.
3. **[MVP]** **Pisni odgovor zdravnika** na konzultacijo v roku 72 ur.
4. **[MVP]** **Določitev termina video klica** s strani zdravnika v roku 12 ur (nujno: 8 ur) in obvestilo pacientu po e-pošti.
5. **[MVP]** **Video klic v brskalniku** (WebRTC) - pacient vstopi prek časovno omejene povezave, brez namestitve aplikacije.
6. **[MVP]** **Digitalno podpisano mnenje** zdravnika, poslano pacientu po e-pošti.
7. **[MVP]** **Samodejni opomniki** zdravniku pred potekom roka in označevanje konzultacij, ki niso bile opravljene pravočasno.
8. Profili zdravnikov: specializacija, delovno mesto, jeziki, cena pisne in video konzultacije, razpoložljivost.
9. Upravljanje provizij zdravnikov z obrazložitvijo spremembe in samodejnim obvestilom zdravniku.
10. Nadzorna plošča s statistiko (prihodki v mesecu, število konzultacij po vrsti in statusu) iz prilagodljivih gradnikov.
11. Dvostopenjska prijava (TOTP) za zdravnike in administratorja.
12. Beleženje poteka video klica (vstop/izstop udeležencev, težave s povezavo) za reševanje reklamacij.
13. Oznaka, da pacient potrebuje prevajalca.

## Kasnejše nadgradnje

- Spletno plačilo s kartico ob naročilu konzultacije.
- Uporabniški račun za pacienta z zgodovino konzultacij in mnenj.
- Koledar razpoložljivosti zdravnika, v katerem pacient sam izbere točen termin.
- Varno sporočanje med pacientom in zdravnikom za dodatna vprašanja po mnenju.
- Ocene in mnenja pacientov o zdravnikih.
- Večjezični uporabniški vmesnik.
- Mobilna aplikacija s potisnimi obvestili.

## Out of scope

- Nujna medicinska pomoč - platforma ni namenjena akutnim stanjem.
- Predpisovanje zdravil (e-recept), napotnice in bolniški stalež.
- Povezava z nacionalnimi sistemi (eZdravje, ZZZS) in obračun prek zdravstvenega zavarovanja.
- Vodenje celotnega elektronskega zdravstvenega kartona pacienta.
- Samodejno (AI) postavljanje diagnoz.
- Snemanje video klicev.
