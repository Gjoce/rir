---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# mise za različice orodij in razvojna opravila

## Kontekst in problem

Za razvoj potrebujemo več orodij (Go, templ, golangci-lint, Hurl, Caddy) in hkrati več procesov (baza, MinIO, Mailpit, Caddy, aplikacija, templ watch). Brez dogovora vsak razvijalec namesti drugačne različice, zagon pa zahteva več terminalov in dolgo navodilo.

## Odločitveni dejavniki

* enako okolje pri vseh razvijalcih,
* zagon celotnega okolja z enim ukazom,
* delovanje na Windows, Linux in macOS,
* čim manj predpogojev.

## Obravnavane alternative

* mise (`mise.toml` za različice orodij in opravila)
* Makefile + ročna namestitev orodij
* Vse v Dockerju (razvoj v kontejnerjih)

## Sprejeta odločitev

Izbrana alternativa: "mise", ker en `mise.toml` določa različice orodij in opravila, zato ima vsak razvijalec enako okolje z `mise install`. Opravilo `develop` poveže vse procese (docker:up, templ:watch, api:run, caddy:run) v en ukaz, opravila pa delujejo enako na Windows, Linux in macOS. Predpogoja sta le mise in Docker.

### Posledice

* Dobro, ker je uvajanje novega razvijalca hitro in ponovljivo.
* Dobro, ker so različice orodij zapisane v repozitoriju in zato dosledne.
* Dobro, ker Make ni potreben, kar odpravi težave na Windows.
* Slabo, ker je mise še ena odvisnost, manj razširjena kot Make.
* Slabo, ker je Docker še vedno potreben za bazo, MinIO in Mailpit.
* Slabo, ker je treba `mise.toml` vzdrževati ob vsaki posodobitvi orodij.

## Prednosti in slabosti alternativ

### Makefile + ročna namestitev

* Dobro, ker je Make poznan.
* Slabo, ker ne upravlja različic orodij in slabo deluje na Windows.

### Vse v Dockerju

* Dobro, ker je okolje izolirano.
* Slabo, ker je počasnejši cikel (gradnja, osveževanje kode) in težje razhroščevanje.

## Dodatne informacije

Povezano: [ADR-0001](0001-monolit-v-go.md).