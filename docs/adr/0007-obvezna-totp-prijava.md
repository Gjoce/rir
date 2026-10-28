---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# Obvezna dvostopenjska prijava (TOTP)

## Kontekst in problem

Zdravniki in administratorji dostopajo do zdravstvenih podatkov pacientov. Ukradeno ali uganjeno geslo ne sme zadostovati za dostop. Poleg tega zdravnikov odgovor sistem digitalno podpiše v njegovem imenu ([ADR-0010](0010-digitalni-podpis-ed25519-kms.md)), zato je pomembno, da ga odda res zdravnik, ne nekdo z dostopom do odprte seje.

## Odločitveni dejavniki

* zaščita občutljivih zdravstvenih podatkov,
* odpornost na krajo gesla,
* uporabnost za zdravnike (poznana aplikacija, brez posebne strojne opreme),
* nizki stroški in brez odvisnosti od zunanje storitve.

## Obravnavane alternative

* Obvezen TOTP (aplikacija za preverjanje pristnosti)
* Samo geslo
* Kode po SMS ali e-pošti
* Strojni ključi (WebAuthn / FIDO2)

## Sprejeta odločitev

Izbrana alternativa: "obvezen TOTP", ker samo geslo ni dovolj za dostop do zdravstvenih podatkov, TOTP pa je standarden, brezplačen in deluje brez zunanje storitve. Ob prvi prijavi mora zdravnik nastaviti TOTP, preden pride do nadzorne plošče. Ista koda ne more biti uporabljena dvakrat. TOTP se dodatno zahteva pri oddaji pisnega odgovora, tudi če je zdravnik že prijavljen, saj je to dejanje podpisano v njegovem imenu.

### Posledice

* Dobro, ker kraja gesla sama ne omogoča dostopa do podatkov.
* Dobro, ker ponovna potrditev pri oddaji odgovora zmanjša tveganje zlorabe odprte seje.
* Dobro, ker ni stroškov SMS in ni odvisnosti od zunanjega ponudnika.
* Slabo, ker mora vsak zdravnik nastaviti aplikacijo in izguba telefona zahteva postopek obnovitve.
* Slabo, ker dodatna koda pri oddaji odgovora rahlo podaljša delo zdravnika.
* Slabo, ker je TOTP manj odporen na ribarjenje kot strojni ključi.
* Slabo, ker je treba varno hraniti `totp_secret` in preprečiti ponovno uporabo kode.

## Prednosti in slabosti alternativ

### Samo geslo

* Dobro, ker je najpreprostejše za uporabnika.
* Slabo, ker je za zdravstvene podatke nezadostno.

### SMS ali e-pošta

* Dobro, ker ne zahteva aplikacije.
* Slabo, ker je manj varno (zamenjava SIM, prestrezanje) in prinaša stroške ali odvisnost.

### Strojni ključi (WebAuthn)

* Dobro, ker je najodpornejše proti ribarjenju.
* Slabo, ker zahteva strojno opremo ali podporo naprav ter zahtevnejšo izvedbo.

## Dodatne informacije

Povezano: US-04, US-15, [ADR-0006](0006-avtorizacija-z-open-policy-agent.md). Strojni ključi so možna kasnejša nadgradnja, ki bi zahtevala nov ADR.