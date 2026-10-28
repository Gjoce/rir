---
status: "sprejeto"
date: 2026-10-28
decision-makers: razvojna ekipa CyberMedico
---

# Strežniško izrisovanje (templ + Datastar) namesto SPA

## Kontekst in problem

Uporabniški vmesnik vključuje obrazce za naročanje, nadzorno ploščo zdravnika, urejevalnik odgovorov in video klic. Potrebujemo interaktivnost (npr. odštevanje, delne posodobitve), a ne želimo vzdrževati ločenega frontend projekta, če to ni nujno.

## Odločitveni dejavniki

* majhna ekipa, en jezik in en projekt,
* logika in avtorizacija naj ostaneta na strežniku (zdravstveni podatki),
* dovolj interaktivnosti brez zapletene gradnje JavaScripta,
* preprosta gradnja in namestitev.

## Obravnavane alternative

* Strežniško izrisan HTML s templ + Datastar (SSE)
* SPA (npr. React) z JSON API-jem
* Klasično strežniško izrisovanje brez interaktivne knjižnice

## Sprejeta odločitev

Izbrana alternativa: "templ + Datastar", ker ostane logika na strežniku, interaktivnost pa je kljub temu mogoča prek strežniških dogodkov (SSE). Ločen frontend projekt in gradnja JavaScripta nista potrebna. Za stil uporabljamo Pico CSS, za specifične potrebe pa manjše knjižnice (Quill, Gridstack, ECharts, Toastify).

### Posledice

* Dobro, ker je en projekt, en jezik in ena gradnja.
* Dobro, ker se predloge `.templ` prevedejo v tipizirano Go kodo, zato napake ujame prevajalnik.
* Dobro, ker občutljivi podatki ne zapuščajo strežnika več, kot je nujno, in avtorizacija je na enem mestu.
* Slabo, ker je ekosistem Datastar in templ manjši kot pri React; manj je primerov in razvijalcev.
* Slabo, ker zelo bogate odjemalske interakcije zahtevajo več dela kot v SPA (video klic je zato izjema z lastnim `video-call.js`).
* Slabo, ker vsaka interakcija zahteva pot do strežnika.

## Prednosti in slabosti alternativ

### SPA (React) z JSON API-jem

* Dobro, ker je ekosistem velik in interaktivnost bogata.
* Slabo, ker zahteva drugi projekt, orodja za gradnjo, upravljanje stanja in podvajanje validacije.
* Slabo, ker je več površine za napake pri izpostavljanju podatkov prek API-ja.

### Klasično strežniško izrisovanje brez interaktivne knjižnice

* Dobro, ker je najpreprostejše.
* Slabo, ker bi odštevanje, nadzorna plošča in delne posodobitve zahtevali veliko ročnega JavaScripta.

## Dodatne informacije

Povezano: [ADR-0003](0003-openapi-in-ogen.md).