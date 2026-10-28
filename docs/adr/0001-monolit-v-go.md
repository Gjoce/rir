---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# Monolit v Go namesto mikrostoritev

## Kontekst in problem

CyberMedico združuje spletne obrazce, avtorizacijo, signalizacijo za video klice in opravila v ozadju. Razvija ga majhna ekipa, ki mora hitro dobiti delujoč MVP (F1–F7). Odločiti se moramo, ali sistem razdelimo na več samostojnih storitev ali ga zgradimo kot en sam proces.

## Odločitveni dejavniki

* majhna ekipa, malo časa za vzdrževanje infrastrukture,
* enostaven razvoj, zagon in razhroščevanje,
* transakcijska konsistentnost med podatki in opravili (opomniki, podpisi),
* preprosta namestitev v produkciji,
* možnost poznejše razdelitve, če bo potrebna.

## Obravnavane alternative

* Monolit v Go (en proces, več delavcev v ozadju)
* Mikrostoritve (npr. ločene storitve za konzultacije, video, obvestila)
* Monolit v drugem ogrodju (npr. Node.js, Java/Spring)

## Sprejeta odločitev

Izbrana alternativa: "Monolit v Go", ker majhna ekipa z njim najhitreje dostavi MVP, en proces pa je najlažje razvijati, zagnati in razhroščevati. Go se prevede v eno izvršljivo datoteko, kar poenostavi namestitev.

### Posledice

* Dobro, ker je en repozitorij, ena gradnja in ena namestitev.
* Dobro, ker ni omrežnih klicev med storitvami, zato ni težav z distribuiranimi transakcijami, različicami API-jev in odkrivanjem storitev.
* Dobro, ker je lokalno razvojno okolje preprosto.
* Slabo, ker se mora celoten sistem skalirati skupaj; video signalizacija in spletni del ne moreta rasti neodvisno.
* Slabo, ker lahko brez discipline meje med moduli sčasoma zbledijo in postane koda prepletena.
* Slabo, ker napaka v enem delu lahko vpliva na ves proces.

### Potrditev

Pri pregledu kode preverjamo, da so moduli (konzultacije, video, opravila, avtorizacija) ločeni v pakete z jasnimi vmesniki.

## Prednosti in slabosti alternativ

### Mikrostoritve

* Dobro, ker omogočajo neodvisno skaliranje in objavljanje delov sistema.
* Slabo, ker zahtevajo veliko infrastrukture (orkestracija, nadzor, omrežje) in zrelo ekipo.
* Slabo, ker so transakcije čez več storitev zapletene, kar je neposredno v nasprotju z zahtevo po zanesljivih opomnikih in podpisih.

### Monolit v drugem ogrodju

* Dobro, ker je ekosistem lahko znan nekaterim članom ekipe.
* Slabo, ker je Go lažji za namestitev (ena datoteka), hiter in ima dobro podporo za sočasnost, ki jo potrebujemo za WebSocket signalizacijo.

## Dodatne informacije

Odločitev ponovno pretehtamo, če bo obremenitev video signalizacije zahtevala neodvisno skaliranje. Povezano: [ADR-0005](0005-river-opravila-v-postgresql.md).