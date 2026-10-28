---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# Avtorizacija z Open Policy Agent (Rego)

## Kontekst in problem

Sistem obdeluje zdravstvene podatke, zato mora biti dostop strogo omejen: zdravnik vidi le svoje konzultacije, odgovori le na svojo pisno konzultacijo v statusu `pending`, administrator ne odgovarja na konzultacije. Pravila dostopa so odvisna od vloge, lastništva in stanja. Če so razpršena po kodi, jih je težko pregledati in zlahka pride do luknje.

## Odločitveni dejavniki

* pravila na enem mestu in berljiva,
* ločena od aplikacijske kode (lažji pregled in revizija),
* podpora pogojem glede vloge, lastništva in statusa,
* testirljivost pravil.

## Obravnavane alternative

* Open Policy Agent s pravili v Rego
* Ročni `if` preveri v handlerjih
* Knjižnica RBAC (npr. Casbin)

## Sprejeta odločitev

Izbrana alternativa: "Open Policy Agent", ker so pravila dostopa zbrana na enem mestu, ločena od kode in berljiva, npr. "zdravnik odgovori le na svojo pisno konzultacijo v statusu `pending`". Vsak zahtevek se preveri glede na vlogo in lastništvo.

### Posledice

* Dobro, ker je pregled pravil mogoč brez branja celotne kode in jih je mogoče testirati ločeno.
* Dobro, ker se pravila spreminjajo brez posegov v poslovno logiko.
* Dobro, ker izrazno moč za pogoje (vloga + lastništvo + status) imamo že vgrajeno.
* Slabo, ker je Rego dodaten jezik, ki se ga mora ekipa naučiti.
* Slabo, ker vsaka odločitev pomeni dodaten klic evalvacije, zato je treba paziti na zmogljivost.
* Slabo, ker je treba aplikacijo in pravila usklajevati (podatki, ki jih pravilo potrebuje, morajo biti podani vhodu).

### Potrditev

Pravila imajo avtomatske teste, HTTP testi (Hurl) pa preverjajo kritične poti, npr. da dostop do tuje konzultacije vrne 403 (US-04).

## Prednosti in slabosti alternativ

### Ročni `if` preveri v handlerjih

* Dobro, ker je najpreprostejše na začetku in brez novih odvisnosti.
* Slabo, ker se pravila razpršijo, pozabljen preveri pomeni varnostno luknjo, pregled je težaven.

### Knjižnica RBAC (Casbin)

* Dobro, ker je enostavnejša za čiste vloge.
* Slabo, ker so naša pravila odvisna tudi od lastništva in statusa konzultacije, kar presega osnovne vloge.

## Dodatne informacije

Povezano: [ADR-0007](0007-obvezna-totp-prijava.md).