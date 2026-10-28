---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# Digitalni podpis mnenj z Ed25519 in AWS KMS

## Kontekst in problem

Pacient mnenje zdravnika pogosto predloži drugemu zdravniku, zato mora biti verodostojno: dokazljivo mora biti, da ga je napisal določen zdravnik in da ga po oddaji nihče ni spremenil. Zasebni ključ zdravnika je izjemno občutljiv in ne sme biti dostopen v čistopisu.

## Odločitveni dejavniki

* celovitost in avtentičnost mnenja (F6),
* zasebni ključ nikoli v čistopisu,
* preprost, hiter in varen podpisni algoritem,
* možnost preverjanja podpisa.

## Obravnavane alternative

* Ed25519 podpis, zasebni ključ zdravnika šifriran z ključem v AWS KMS
* Podpis s skupnim ključem platforme
* Hramba zasebnih ključev v čistopisu v bazi
* Kvalificirano elektronsko potrdilo zunanjega ponudnika

## Sprejeta odločitev

Izbrana alternativa: "Ed25519 + KMS", ker podpis dokazuje, da mnenja po oddaji nihče ni spremenil, zasebni ključ zdravnika pa je šifriran s ključem v KMS in nikoli ni shranjen v čistopisu (`users.encrypted_private_key`, `users.public_key`). Podpis zajema vse podatke konzultacije in odgovor, zato sprememba katerega koli polja povzroči neuspešno preverjanje (US-09). Podpisovanje izvede opravilo v ozadju, podpis pa se prilepi mnenju ob oddaji z zdravnikovo potrditvijo s TOTP ([ADR-0007](0007-obvezna-totp-prijava.md)).

### Posledice

* Dobro, ker je podpis vezan na posameznega zdravnika, ne na platformo, in je mogoče zaznati vsako spremembo.
* Dobro, ker uhajanje baze samo po sebi ne razkrije zasebnih ključev (potreben je še KMS).
* Dobro, ker je Ed25519 hiter, ima kratke ključe in podpise ter je dobro podprt.
* Slabo, ker smo odvisni od AWS KMS (razpoložljivost, strošek, vezanost na ponudnika).
* Slabo, ker ni kvalificirano elektronsko potrdilo, zato ima podpis pravno težo le toliko, kot jo prizna prejemnik; to ni nadomestilo za pravno veljaven e-podpis.
* Slabo, ker izguba ali rotacija ključev KMS zahteva premišljen postopek in preverjanje starih podpisov mora ostati mogoče.

## Prednosti in slabosti alternativ

### Skupni ključ platforme

* Dobro, ker je najpreprostejše upravljanje.
* Slabo, ker podpis ne dokazuje, kateri zdravnik je mnenje napisal, kompromitacija ključa pa prizadene vse.

### Ključi v čistopisu

* Dobro, ker ni odvisnosti od KMS.
* Slabo, ker uhajanje baze razkrije ključe in omogoča ponarejanje.

### Kvalificirano potrdilo

* Dobro, ker je pravno najmočnejše.
* Slabo, ker je drago, zapleteno za uvedbo in presega obseg MVP.

## Dodatne informacije

Povezano: US-09, [ADR-0005](0005-river-opravila-v-postgresql.md), [ADR-0007](0007-obvezna-totp-prijava.md). Če bo potreben pravno veljaven e-podpis, to zahteva nov ADR.