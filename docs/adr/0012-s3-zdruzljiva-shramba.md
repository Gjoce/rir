---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# S3-združljiva objektna shramba za datoteke

## Kontekst in problem

Pacienti prilagajo izvide (do 5 datotek, PDF/JPG/PNG, do 10 MB vsaka), sistem pa hrani tudi podpise mnenj. Odločiti se moramo, kje hraniti datoteke.

## Odločitveni dejavniki

* datoteke naj ne obremenjujejo baze,
* ista koda lokalno in v produkciji,
* neodvisnost od enega ponudnika,
* preprosto lokalno razvojno okolje.

## Obravnavane alternative

* S3-združljiva shramba (lokalno MinIO, v produkciji katerikoli S3 ponudnik)
* Datoteke v PostgreSQL (`bytea` ali large objects)
* Lokalni datotečni sistem strežnika

## Sprejeta odločitev

Izbrana alternativa: "S3-združljiva shramba", ker datoteke niso v bazi, lokalno uporabljamo MinIO, v produkciji pa lahko izberemo katerega koli S3 ponudnika brez spremembe kode. V bazi (`consultation_attachments`) so le metapodatki (`s3_key`, `original_filename`, `content_type`, `size_bytes`).

### Posledice

* Dobro, ker je baza manjša in varnostne kopije hitrejše.
* Dobro, ker se shramba skalira neodvisno od aplikacije.
* Dobro, ker ni vezave na enega ponudnika zaradi standardnega S3 API-ja.
* Slabo, ker je treba skrbeti za doslednost med bazo in shrambo (npr. osirotele datoteke ob napaki).
* Slabo, ker je dodatna komponenta, ki jo je treba zaščititi (dostop, šifriranje), saj gre za zdravstvene dokumente.
* Slabo, ker varnostna kopija zahteva usklajevanje baze in shrambe.

## Prednosti in slabosti alternativ

### Datoteke v PostgreSQL

* Dobro, ker so podatki in datoteke v isti transakciji ter ena varnostna kopija.
* Slabo, ker napihne bazo, upočasni varnostne kopije in obremeni povezave.

### Lokalni datotečni sistem

* Dobro, ker je najpreprostejši.
* Slabo, ker otežuje skaliranje in več instanc, datoteke pa so vezane na strežnik.

## Dodatne informacije

Povezano: [ADR-0004](0004-postgresql-kot-glavna-baza.md).