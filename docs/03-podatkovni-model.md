# Podatkovni model

Podatki so shranjeni v relacijski bazi **PostgreSQL**. Priloge (izvidi) so shranjene v objektni shrambi (S3/MinIO), v bazi so le njihovi metapodatki. Vse tabele imajo še polji `created_at` in `updated_at`, ki ju spodaj ne navajamo.

## Entitete in glavna polja

| Entiteta | Namen | Glavna polja |
|---|---|---|
| **users** | Prijavljeni uporabniki (zdravniki, administratorji) | `id`, `email`, `password_hash`, `name`, `totp_secret`, `totp_enrolled`, `public_key`, `encrypted_private_key` |
| **roles** | Vloge v sistemu (`admin`, `doctor`) | `id` |
| **users_roles** | Dodelitev vlog uporabnikom | `user_id`, `role_id` |
| **doctor_profile** | Javni profil zdravnika | `id` (= `users.id`), `specialization`, `subspecialization`, `display_name`, `bio`, `workplace`, `available`, `consultation_price_written`, `consultation_price_video` |
| **doctor_financials** | Finančni podatki zdravnika | `doctor_id`, `bank_account`, `provision`, `provision_change_reason` |
| **doctor_contacts** | Kontaktni podatki zdravnika | `doctor_id`, `phone_number`, `meeting_link` |
| **languages** / **doctor_language** | Jeziki, ki jih zdravnik govori | `id` / `doctor_id`, `language_id` |
| **consultations** | Skupni podatki vsake konzultacije | `id`, `doctor_id`, `consultation_type` (`written`/`video`), `consultation_status` (`scheduled`/`pending`/`finished`/`not_attended`), `consultation_payment_status`, `price`, `urgent`, `need_translator`, podatki pacienta (`patient_name`, `patient_surname`, `patient_age`, `patient_email`, `patient_phone`), `medical_problem`, `patient_diagnosis`, `allergies`, `medication`, `signature_doctor_url`, `finished_at` |
| **written_consultations** | Podrobnosti pisne konzultacije | `consultation_id`, `doctor_answer`, `expires_at` (+72 h), `email_reminder_sent` |
| **video_consultations** | Podrobnosti video konzultacije | `consultation_id`, `preferred_time` (`morning`/`afternoon`/`evening`), `scheduled_at`, `requires_response_by_schedule` (+12 h, nujno +8 h), `expires_at_after_video` (+24 h), `doctor_answer_after_video` |
| **consultation_attachments** | Priloge pacienta | `id`, `consultation_id`, `s3_key`, `original_filename`, `content_type`, `size_bytes` |
| **consultation_call_events** | Dnevnik dogodkov video klica | `id`, `consultation_id`, `kind` (npr. `patient_joined`, `ice_failed`, `call_ended`), `occurred_at`, `detail` |
| **dashboard_widgets** | Gradniki nadzorne plošče za posamezno vlogo | `id`, `role`, `title`, `type`, `config`, `layout` |
| **sessions** | Seje prijavljenih uporabnikov | `token`, `data`, `expiry` |

Pacient nima uporabniškega računa, zato so njegovi podatki shranjeni neposredno v `consultations`.

## Povezave med entitetami

- **users - roles**: M:N prek `users_roles`.
- **users - doctor_profile / doctor_financials / doctor_contacts**: 1:1 (le za zdravnike).
- **users - languages**: M:N prek `doctor_language`.
- **users (zdravnik) - consultations**: 1:N - zdravnik ima več konzultacij, vsaka konzultacija pripada enemu zdravniku.
- **consultations - written_consultations / video_consultations**: 1:1 - glede na `consultation_type` ima konzultacija natanko eno od obeh razširitev.
- **consultations - consultation_attachments**: 1:N.
- **consultations - consultation_call_events**: 1:N (le za video konzultacije).
- **roles - dashboard_widgets**: 1:N.

Roke (`expires_at`, `requires_response_by_schedule`, `expires_at_after_video`) samodejno izračunajo prožilci (triggerji) v bazi.

## Osnutek podatkovnega modela

```mermaid
erDiagram
    users ||--o{ users_roles : ima
    roles ||--o{ users_roles : dodeljena
    roles ||--o{ dashboard_widgets : prikazuje
    users ||--o| doctor_profile : ima
    users ||--o| doctor_financials : ima
    users ||--o| doctor_contacts : ima
    users ||--o{ doctor_language : govori
    languages ||--o{ doctor_language : ""
    users ||--o{ consultations : obravnava
    consultations ||--o| written_consultations : "je pisna"
    consultations ||--o| video_consultations : "je video"
    consultations ||--o{ consultation_attachments : vsebuje
    consultations ||--o{ consultation_call_events : belezi

    users {
        uuid id PK
        text email
        text password_hash
        text totp_secret
        bytea public_key
    }
    roles {
        text id PK
    }
    users_roles {
        uuid user_id FK
        text role_id FK
    }
    doctor_profile {
        uuid id PK
        text specialization
        text display_name
        bool available
        numeric consultation_price_written
        numeric consultation_price_video
    }
    doctor_financials {
        uuid doctor_id PK
        text bank_account
        int provision
    }
    doctor_contacts {
        uuid doctor_id PK
        text phone_number
    }
    languages {
        text id PK
    }
    doctor_language {
        uuid doctor_id FK
        text language_id FK
    }
    consultations {
        uuid id PK
        uuid doctor_id FK
        enum consultation_type
        enum consultation_status
        enum consultation_payment_status
        numeric price
        bool urgent
        text patient_email
        text medical_problem
    }
    written_consultations {
        uuid consultation_id PK
        text doctor_answer
        timestamptz expires_at
    }
    video_consultations {
        uuid consultation_id PK
        enum preferred_time
        timestamptz scheduled_at
        timestamptz requires_response_by_schedule
        text doctor_answer_after_video
    }
    consultation_attachments {
        uuid id PK
        uuid consultation_id FK
        text s3_key
        text original_filename
    }
    consultation_call_events {
        bigint id PK
        uuid consultation_id FK
        enum kind
        timestamptz occurred_at
    }
    dashboard_widgets {
        uuid id PK
        text role FK
        text type
        jsonb config
    }
```
