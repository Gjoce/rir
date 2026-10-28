---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# Pacient brez računa, vstop v video klic s HMAC žetonom

## Kontekst in problem

Pacient je običajno enkratni uporabnik, ki želi mnenje čim hitreje. Registracija in prijava sta trenje, ki lahko odvrne od naročila. Hkrati mora biti vstop v video klic zaščiten: klicu se sme pridružiti samo pacient te konzultacije, in to samo v času termina.

## Odločitveni dejavniki

* čim manj trenja za pacienta,
* varen vstop, omejen na eno konzultacijo in čas termina,
* manj shranjenih osebnih podatkov in manj upravljanja računov,
* preprostost (brez zunanjega ponudnika identitete).

## Obravnavane alternative

* Brez računa, vstop s podpisanim časovno omejenim žetonom (HMAC) v povezavi iz e-pošte
* Obvezna registracija pacienta z računom
* Enostavna povezava brez podpisa (npr. samo ID konzultacije)

## Sprejeta odločitev

Izbrana alternativa: "brez računa + HMAC žeton", ker je trenje za enkratnega uporabnika najmanjše, žeton pa je podpisan, vezan na eno konzultacijo in časovno omejen. Pacientovi podatki so shranjeni neposredno v `consultations`. Povezava omogoča vstop od 10 minut pred do 60 minut po terminu (US-07).

### Posledice

* Dobro, ker pacient ne ustvarja računa in gesla.
* Dobro, ker povezava s spremenjenim ID-jem konzultacije ne deluje (US-06).
* Dobro, ker ni potrebe po upravljanju pacientovih računov, ponastavitvah gesel ali dodatni TOTP za paciente.
* Slabo, ker kdor prestreže ali posreduje povezavo, lahko v oknu termina vstopi; varnost je vezana na zaupnost e-pošte.
* Slabo, ker pacient nima zgodovine konzultacij; to je predvidena kasnejša nadgradnja (uporabniški račun).
* Slabo, ker so potrebni skrbno upravljanje skrivnosti HMAC in omejitev števila udeležencev v sobi (največ dva).

## Prednosti in slabosti alternativ

### Obvezna registracija

* Dobro, ker omogoča zgodovino in močnejšo identifikacijo.
* Slabo, ker povečuje trenje, razvoj in odgovornost za račune.

### Povezava brez podpisa

* Dobro, ker je najpreprostejša.
* Slabo, ker je ID mogoče ugibati ali spreminjati, kar je nesprejemljivo za zdravstveni klic.

## Dodatne informacije

Povezano: US-06, US-07, [ADR-0009](0009-webrtc-peer-to-peer.md). Če uvedemo račune za paciente, to zahteva nov ADR, ki nadomesti ali dopolni ta zapis.