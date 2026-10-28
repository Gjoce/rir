---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# PostgreSQL kot glavna podatkovna baza

## Kontekst in problem

Podatki so izrazito relacijski (zdravnik, konzultacija, priloge, dogodki klica), potrebujemo transakcije, rokov pa ne želimo računati v aplikacijski kodi. Poleg poslovnih podatkov moramo nekje hraniti tudi seje in čakalno vrsto opravil.

## Odločitveni dejavniki

* relacijski podatkovni model z omejitvami in tujimi ključi,
* transakcijska konsistentnost,
* čim manj dodatne infrastrukture,
* podpora za tipe, kot so `timestamptz`, `jsonb`, enumi in prožilci.

## Obravnavane alternative

* PostgreSQL (podatki, seje, čakalna vrsta)
* PostgreSQL za podatke + Redis za seje in čakalno vrsto
* Dokumentna baza (npr. MongoDB)

## Sprejeta odločitev

Izbrana alternativa: "PostgreSQL za vse", ker so podatki relacijski in potrebujemo transakcije, ista baza pa lahko služi tudi za seje (scs + pgxstore) in čakalno vrsto (River), kar zmanjša infrastrukturo. Roke konzultacij (`expires_at`, `requires_response_by_schedule`, `expires_at_after_video`) računajo prožilci v bazi, zato so pravila na enem mestu in veljajo ne glede na to, kdo podatke vstavi. Shema se upravlja z migracijami (golang-migrate), ki se ob zagonu izvedejo samodejno.

### Posledice

* Dobro, ker so omejitve, tuji ključi in transakcije zagotovljene na ravni baze.
* Dobro, ker je za delovanje potrebna ena sama baza, varnostne kopije pa zajamejo tudi seje in opravila.
* Dobro, ker prožilci zagotavljajo dosledne roke.
* Slabo, ker del poslovne logike (roki) živi v bazi in ga je težje testirati ter razhroščevati kot aplikacijsko kodo.
* Slabo, ker se seje in opravila delijo vire z glavnimi podatki; ob veliki obremenitvi bi lahko vplivali na zmogljivost.
* Slabo, ker samodejne migracije ob zagonu zahtevajo previdnost pri nadgradnjah z več instancami.

## Prednosti in slabosti alternativ

### PostgreSQL + Redis

* Dobro, ker je Redis zelo hiter za seje.
* Slabo, ker dodaja novo infrastrukturo in izgubi transakcijsko povezavo z opravili (glej [ADR-0005](0005-river-opravila-v-postgresql.md)).

### Dokumentna baza

* Dobro, ker je shema prožna.
* Slabo, ker so naši podatki relacijski, omejitve in transakcije pa bi morali nadomeščati v kodi.

## Dodatne informacije

Podrobnosti modela: [03-podatkovni-model.md](../03-podatkovni-model.md).