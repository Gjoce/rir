---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# Video klic prek WebRTC peer-to-peer s TURN rezervo

## Kontekst in problem

Video konzultacija mora delovati v brskalniku brez namestitve aplikacije (F5). Treba je izbrati, kako se prenaša video: prek lastnega strežnika, prek zunanje platforme ali neposredno med brskalnikoma.

## Odločitveni dejavniki

* brez namestitve za pacienta,
* zasebnost pogovora (zdravstveni podatki),
* obremenitev in stroški strežnika,
* zanesljivost za uporabnike za NAT in požarnimi zidovi.

## Obravnavane alternative

* WebRTC peer-to-peer, signalizacija prek WebSocket, TURN le kot rezerva
* Medijski strežnik (SFU/MCU), prek katerega teče ves video
* Zunanja video storitev (SaaS, npr. Zoom ali Jitsi kot storitev)

## Sprejeta odločitev

Izbrana alternativa: "WebRTC peer-to-peer s TURN rezervo", ker video ne teče prek našega strežnika, kar zmanjša obremenitev in izboljša zasebnost. Naša aplikacija posreduje le signalizacijo (SDP/ICE prek WebSocket, coder/websocket), medije pa si brskalnika izmenjata neposredno. Če neposredna povezava ni mogoča, se klic samodejno vzpostavi prek TURN (coturn), brez dejanja pacienta.

### Posledice

* Dobro, ker strežnik ne obdeluje video prometa, razen ko je potreben TURN.
* Dobro, ker je zasebnost boljša: mediji gredo neposredno med udeležencema in so šifrirani.
* Dobro, ker ni stroškov zunanje video storitve in ni podatkov pri tretji osebi.
* Slabo, ker je za zanesljivost za NAT treba upravljati STUN/TURN (coturn), kar je dodatna komponenta.
* Slabo, ker promet prek TURN porabi pasovno širino strežnika.
* Slabo, ker je P2P primeren za dva udeleženca (zadošča, saj sta v klicu največ dva), večjih skupin ne podpira.
* Slabo, ker nimamo snemanja na strežniku (je tudi out of scope) in imamo manj vpogleda v kakovost; to deloma blažimo z dnevnikom dogodkov `consultation_call_events`.

## Prednosti in slabosti alternativ

### Medijski strežnik (SFU/MCU)

* Dobro, ker omogoča več udeležencev, snemanje in enotno kakovost.
* Slabo, ker je zahteven za delovanje, drag in ves zdravstveni video teče prek nas.

### Zunanja video storitev

* Dobro, ker je hitro uvedljiva in zanesljiva.
* Slabo, ker pogovor teče prek tretje osebe, kar je za zdravstvene podatke problematično, in dodaja stroške ter odvisnost.

## Dodatne informacije

Povezano: [ADR-0008](0008-pacient-brez-racuna-zeton-hmac.md), US-07, US-08. Snemanje klicev je izven obsega (out of scope).