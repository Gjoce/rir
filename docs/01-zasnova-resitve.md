# Zasnova rešitve

## Ime rešitve

**CyberMedico**

## Namen in problem

Pacient, ki želi mnenje specialista, pogosto čaka več tednov ali mora potovati v drug kraj. CyberMedico mu omogoča, da od doma naroči spletno konzultacijo pri izbranem specialistu in mnenje prejme v nekaj dneh, brez obiska ambulante.

## Ciljni uporabniki

- **Pacient** - ima zdravstveno težavo ali že postavljeno diagnozo in želi (drugo) mnenje specialista, ne da bi čakal na pregled.
- **Zdravnik specialist** - prek spleta svetuje pacientom ob svojem rednem delu.
- **Administrator** - vodi platformo: dodaja zdravnike, določa njihove provizije in spremlja statistiko.

## Kratek opis delovanja

**Pacient** (doma, brez registracije):

1. Izbere specialista in vrsto konzultacije:
   - **pisna** - opiše težavo in prejme pisno mnenje,
   - **video** - pogovor z zdravnikom prek video klica v brskalniku.
2. Izpolni obrazec (opis težave, diagnoza, zdravila, alergije, izvidi kot priloge), po potrebi označi nujnost ali potrebo po prevajalcu, in plača.
3. Mnenje zdravnika prejme po e-pošti, digitalno podpisano.

**Zdravnik** (prijava v nadzorno ploščo):

1. Vidi nove konzultacije, ki so mu dodeljene.
2. Pri pisni konzultaciji odda odgovor v **72 urah**.
3. Pri video konzultaciji določi termin v **12 urah** (nujno: **8 ur**), izvede klic in v **24 urah** po klicu odda pisno mnenje.

**Sistem** samodejno pošilja opomnike po e-pošti in označi konzultacije, ki niso bile opravljene v roku.
