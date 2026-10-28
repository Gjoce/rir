# CyberMedico

Platforma za spletne konzultacije s specialisti. Pacient, ki želi mnenje specialista, pogosto čaka več tednov ali mora potovati v drug kraj. CyberMedico mu omogoča, da od doma naroči konzultacijo pri izbranem zdravniku in mnenje prejme v nekaj dneh, brez obiska ambulante.

## Kako deluje

- **Pacient** (brez registracije) izbere specialista, izpolni obrazec s prilogami in naroči **pisno** ali **video** konzultacijo. Mnenje prejme po e-pošti, digitalno podpisano.
- **Zdravnik** se prijavi z geslom in TOTP kodo, odgovori na pisno konzultacijo v 72 urah ali določi termin video klica v 12 urah (nujno: 8 ur) ter po klicu odda mnenje v 24 urah.
- **Administrator** dodaja zdravnike, določa provizije in spremlja statistiko.
- **Sistem** samodejno pošilja opomnike in označi konzultacije, ki niso bile opravljene v roku.

## Uporabljene tehnologije

| Področje | Tehnologije |
|---|---|
| Frontend | templ, Datastar, Pico CSS, Quill, Gridstack, ECharts, WebRTC |
| Backend | Go, `net/http`, ogen (OpenAPI), River (opravila v ozadju) |
| Podatki | PostgreSQL, pgx, golang-migrate, S3 / MinIO |
| Varnost | bcrypt, TOTP, Open Policy Agent (Rego), HMAC žetoni, Ed25519 + AWS KMS |
| Razvoj | mise, Docker Compose, Caddy, Mailpit, golangci-lint, Hurl |
| Video | WebRTC, WebSocket signalizacija, coturn (TURN) |

## Hiter zagon

Predpogoja sta [mise](https://mise.jdx.dev/) in Docker.

```bash
mise trust
mise install
mise run develop
```

- Aplikacija: `http://localhost:8080` oziroma `https://cybermedico.local` (prek Caddy)
- Poslana e-pošta (Mailpit): `http://localhost:8025`
- HTTP testi: `mise run test:hurl`
- Vsa opravila: `mise tasks`

## Dokumentacija

Vsa dokumentacija je v mapi [`docs/`](docs/):

- [Funkcionalnosti in obseg](docs/02-funkcionalnosti-in-obseg.md)
- [Podatkovni model](docs/03-podatkovni-model.md)
- [Arhitekturne odločitve (ADR)](docs/adr/README.md)