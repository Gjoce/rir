---
name: write-tests
description: Uporabi, ko pišeš ali dopolnjuješ teste v CyberMedico – Go enotske teste, Rego teste avtorizacijskih pravil ali Hurl HTTP teste. Opisuje konvencije projekta, kam gre kateri test in kako ga zagnati.
---

# Pisanje testov po konvencijah projekta

Projekt ima tri vrste testov. Izberi glede na to, kaj testiraš:

| Kaj testiraš | Vrsta | Kje | Zagon |
|---|---|---|---|
| čisto logiko (čas, roki, žetoni, podpisi, validacija) | Go | `*_test.go` ob kodi | `mise exec -- go test ./...` |
| kdo sme kaj (vloge, statusi) | Rego | `authorization/policies_test/*_test.rego` | `mise run test:policies` |
| HTTP odziv (status, preusmeritev, vsebina) | Hurl | `tests/hurl/*.hurl` | `mise run test:hurl` (aplikacija mora teči) |

## Go testi

- Samo standardni paket `testing`, brez testify ali drugih knjižnic.
- Isti paket kot koda (`package util`, ne `util_test`), da lahko testiraš neizvožene funkcije.
- **Tabelarični testi** s `t.Run` za več primerov:

```go
func TestIsWithinJoinWindow(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"exactly 10 minutes before", scheduled.Add(-10 * time.Minute), true},
		{"exactly 60 minutes after", scheduled.Add(60 * time.Minute), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsWithinJoinWindow(scheduled, tt.now); got != tt.want {
				t.Errorf("IsWithinJoinWindow(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}
```

- Imena: `TestFunkcija` ali `TestFunkcijaPrimer`; opis primera v angleščini.
- Časi so fiksni (`time.Date(...)`), nikoli `time.Now()` v pričakovani vrednosti.
- Testiraj **meje** (točno na roku, minuto pred, minuto po) in prazne/ničelne vrednosti.
- Časovni pasovi: dodaj `_ "time/tzdata"` v uvoze, če test uporablja `Europe/Skopje`.
- Sporočilo napake: `t.Errorf("Funkcija(vhod) = %v, want %v", got, want)`.
- Testi ne smejo potrebovati baze, S3 ali omrežja; tako logiko izloči v čisto funkcijo.
- Za HTTP pomožne funkcije uporabi `httptest.NewRequest` in `httptest.NewRecorder`.

Primeri: `cmd/cybermedico/api/util/util_test.go`, `video/tokens_test.go`, `cmd/cybermedico/signing/signing_test.go`.

## Rego testi

- `package policies_test`, `import data.policies`.
- Pomožne vrednosti (`request`, `admin`, `doctor`, `anonymous`, `no_target`) so v `helpers_test.rego`; uporabi jih.
- Za vsako novo operacijo vsaj: dovoljeno za pravo vlogo, zavrnjeno za anonimnega, zavrnjeno za napačno vlogo ali tuj vir.

```rego
test_doctor_sees_dashboard if {
	policies.allow with input as request("view_dashboard", doctor, no_target)
}

test_anonymous_cannot_see_dashboard if {
	not policies.allow with input as request("view_dashboard", anonymous, no_target)
}
```

## Hurl testi

- Ena datoteka na področje (`tests/hurl/<področje>.hurl`), komentar `#` nad vsakim primerom pove, kaj preverja.
- Host vedno kot `{{host}}`.
- Strani za prijavljene: brez prijave pričakuj `HTTP 303` in `header "Location" == "/auth/login"`.

```hurl
# The dashboard needs a login
GET {{host}}/dashboard
HTTP 303
[Asserts]
header "Location" == "/auth/login"
```

## Pred oddajo

```
mise exec -- golangci-lint run ./...
mise exec -- go test ./...
mise run test:policies
```

Vsi morajo biti zeleni. Testa, ki pade, ne izklapljaj in ga ne briši – popravi kodo ali test in razloži, zakaj.
