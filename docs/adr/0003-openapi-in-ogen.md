---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# OpenAPI specifikacija in generiranje strežnika z ogen

## Kontekst in problem

Aplikacija izpostavlja veliko HTTP končnih točk (obrazci, konzultacije, video, administracija). Če se koda in dokumentacija API-ja razhajata, nastajajo napake in nejasnosti. Odločiti se moramo, kako opisujemo in implementiramo API.

## Odločitveni dejavniki

* ena sama resnica o API-ju,
* tipska varnost in manj ročno napisane kode za usmerjanje in razčlenjevanje,
* združljivost s standardnim `net/http`,
* možnost testiranja (Hurl) glede na specifikacijo.

## Obravnavane alternative

* OpenAPI + ogen (strežnik generiran iz specifikacije)
* Ročno napisani handlerji nad `net/http` z ločeno dokumentacijo
* Drugo ogrodje za usmerjanje (npr. chi, gin) brez generiranja

## Sprejeta odločitev

Izbrana alternativa: "OpenAPI + ogen", ker je API opisan v eni specifikaciji, iz katere se generirajo tipi in usmerjanje, zato se koda in specifikacija ne morejo razhajati.

### Posledice

* Dobro, ker so zahtevki in odgovori tipizirani, validacija vhodnih podatkov pa sledi specifikaciji.
* Dobro, ker je specifikacija vedno aktualna dokumentacija.
* Slabo, ker je treba pred zagonom poganjati generator in se držati njegovih omejitev.
* Slabo, ker nenavadni primeri (SSE, WebSocket, strežniško izrisan HTML) niso naravno opisljivi v OpenAPI in jih obravnavamo ločeno.
* Slabo, ker generirana koda povečuje repozitorij in otežuje nekatere razhroščevalne situacije.

## Prednosti in slabosti alternativ

### Ročni handlerji nad `net/http`

* Dobro, ker je največ nadzora in ni generiranja.
* Slabo, ker se dokumentacija hitro zastara in je veliko ponavljajoče se kode.

### Ogrodje za usmerjanje brez generiranja

* Dobro, ker je prilagodljivo in dobro poznano.
* Slabo, ker ne rešuje problema razhajanja med kodo in specifikacijo.

## Dodatne informacije

Povezano: [ADR-0001](0001-monolit-v-go.md), [ADR-0002](0002-streznisko-izrisovanje-templ-datastar.md).