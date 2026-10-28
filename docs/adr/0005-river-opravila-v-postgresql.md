---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# River v PostgreSQL za opravila v ozadju

## Kontekst in problem

Sistem mora zanesljivo izvajati opravila v ozadju: opomnike po e-pošti (US-10), označevanje neopravljenih konzultacij (US-11), podpisovanje mnenj in pošiljanje podpisane e-pošte (US-09) ter obvestila o spremembi provizije (US-13). Izguba opravila pomeni zamujen opomnik ali neposlano mnenje, podvojeno opravilo pa lahko pomeni dvakrat poslano e-pošto.

## Odločitveni dejavniki

* opravilo se ne sme izgubiti, če se spremeni podatek (in obratno),
* idempotentnost (e-pošta natanko enkrat),
* čim manj dodatne infrastrukture,
* vidnost in nadzor nad opravili.

## Obravnavane alternative

* River (čakalna vrsta v PostgreSQL, embedded)
* Zunanji posrednik sporočil (RabbitMQ, Redis)
* Cron + ročno poizvedovanje po bazi

## Sprejeta odločitev

Izbrana alternativa: "River v PostgreSQL", ker se opravilo vpiše v isti transakciji kot sprememba podatkov. Zato ne more nastati stanje, kjer je konzultacija spremenjena, opomnik pa ni zabeležen, ali obratno. Dodatna infrastruktura (Redis, RabbitMQ) ni potrebna, River UI pa omogoča pregled opravil.

### Posledice

* Dobro, ker ni izgubljenih opomnikov zaradi razhajanja med bazo in čakalno vrsto.
* Dobro, ker je operativno manj komponent in ena varnostna kopija.
* Dobro, ker so ponovni poskusi in pregled opravil vgrajeni.
* Slabo, ker so opravila obremenitev PostgreSQL; pri zelo velikem obsegu bi bil ločen posrednik primernejši.
* Slabo, ker smo vezani na River in PostgreSQL.
* Slabo, ker so potrebna idempotentna opravila (npr. zastavica `email_sent_patient`), saj River lahko isto opravilo obdela večkrat.

### Potrditev

Testi preverijo, da ponovna obdelava opravila ne pošlje druge e-pošte (US-09) in da opomnik ni poslan, če je zdravnik že odgovoril (US-10).

## Prednosti in slabosti alternativ

### Zunanji posrednik sporočil

* Dobro, ker je namenjen velikemu obsegu in ločen od baze.
* Slabo, ker zapis v bazo in objava sporočila nista atomarna, kar zahteva dodatne vzorce (npr. outbox).
* Slabo, ker pomeni dodatno infrastrukturo za majhno ekipo.

### Cron + poizvedovanje

* Dobro, ker je zelo preprosto.
* Slabo, ker nima ponovnih poskusov, nadzora in natančnega časovnega sprožanja, hitro pa postane težko vzdrževati.

## Dodatne informacije

Povezano: [ADR-0004](0004-postgresql-kot-glavna-baza.md).