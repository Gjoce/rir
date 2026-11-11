-- Local development data only. Every account uses the password cybermedico-dev
-- and shares the TOTP secret below. Never use these values outside local development.

TRUNCATE TABLE consultation_call_events, consultation_attachments, dashboard_widgets, users, users_roles, roles, consultations, doctor_profile, doctor_financials, doctor_contacts, doctor_language, languages, written_consultations, video_consultations;

INSERT INTO users (id, email, password_hash, name, totp_secret, totp_enrolled) VALUES
('11111111-1111-1111-1111-111111111111', 'admin@cybermedico.com',     '$2a$10$f9tXV9Fy.GT89aMEYnvtSebuHMgTZyGre6jDoTtVu/codgWfW8Daq', 'Admin', 'OZ3LXYGEXTSB6YN2OFZZZSOAUS6TEWOP', true),
('22222222-2222-2222-2222-222222222221', 'dr.carter@cybermedico.com', '$2a$10$f9tXV9Fy.GT89aMEYnvtSebuHMgTZyGre6jDoTtVu/codgWfW8Daq', 'Emily Carter', 'OZ3LXYGEXTSB6YN2OFZZZSOAUS6TEWOP', true),
('22222222-2222-2222-2222-222222222222', 'dr.wilson@cybermedico.com', '$2a$10$f9tXV9Fy.GT89aMEYnvtSebuHMgTZyGre6jDoTtVu/codgWfW8Daq', 'James Wilson', 'OZ3LXYGEXTSB6YN2OFZZZSOAUS6TEWOP', true),
('22222222-2222-2222-2222-222222222223', 'dr.mitchell@cybermedico.com', '$2a$10$f9tXV9Fy.GT89aMEYnvtSebuHMgTZyGre6jDoTtVu/codgWfW8Daq', 'Sarah Mitchell', 'OZ3LXYGEXTSB6YN2OFZZZSOAUS6TEWOP', true),
('22222222-2222-2222-2222-222222222224', 'dr.bennett@cybermedico.com',  '$2a$10$f9tXV9Fy.GT89aMEYnvtSebuHMgTZyGre6jDoTtVu/codgWfW8Daq', 'Michael Bennett', 'OZ3LXYGEXTSB6YN2OFZZZSOAUS6TEWOP', true),
('33333333-3333-3333-3333-333333333333', 'patient@example.com',          '$2a$10$f9tXV9Fy.GT89aMEYnvtSebuHMgTZyGre6jDoTtVu/codgWfW8Daq', 'Patient', NULL, false),
('22222222-2222-2222-2222-222222222225', 'test@cybermedico.com', '$2a$10$f9tXV9Fy.GT89aMEYnvtSebuHMgTZyGre6jDoTtVu/codgWfW8Daq', 'Sarah Mitchell', 'OZ3LXYGEXTSB6YN2OFZZZSOAUS6TEWOP', true);

INSERT INTO roles (id) VALUES ('admin'), ('doctor');

INSERT INTO users_roles (user_id, role_id) VALUES
('11111111-1111-1111-1111-111111111111', 'admin'),
('22222222-2222-2222-2222-222222222221', 'doctor'),
('22222222-2222-2222-2222-222222222222', 'doctor'),
('22222222-2222-2222-2222-222222222223', 'doctor'),
('22222222-2222-2222-2222-222222222225', 'doctor'),
('22222222-2222-2222-2222-222222222224', 'doctor');

INSERT INTO doctor_profile (id, specialization, subspecialization, display_name, photo_url, bio, workplace, available, consultation_price_video, consultation_price_written) VALUES
(
    '22222222-2222-2222-2222-222222222221',
    'Cardiology', 'Interventional Cardiology',
    'Dr. Emily Carter',
    '/static/doctors/22222222-2222-2222-2222-222222222221.jpg',
    'Board-certified cardiologist with 14 years of experience in interventional cardiology. Graduated from the Ss. Cyril and Methodius University School of Medicine. Fellow of the European Society of Cardiology. Special interest in coronary artery disease, heart failure, and preventive cardiology.',
    'University Clinic for Cardiology, Skopje',
    TRUE, 150.00, 70.00
),
(
    '22222222-2222-2222-2222-222222222222',
    'Neurology', 'Headache and Pain Medicine',
    'Dr. James Wilson',
    '/static/doctors/22222222-2222-2222-2222-222222222222.jpg',
    'Neurologist specialising in headache disorders, epilepsy, and multiple sclerosis with over 10 years of clinical practice. Completed advanced training at the University Hospital of Ljubljana. Active member of the European Academy of Neurology.',
    'University Clinic for Neurology, Skopje',
    TRUE, 130.00, 60.00
),
(
    '22222222-2222-2222-2222-222222222223',
    'Endocrinology', 'Diabetes and Thyroid Disorders',
    'Dr. Sarah Mitchell',
    '/static/doctors/22222222-2222-2222-2222-222222222223.jpg',
    'Endocrinologist with 9 years of experience managing diabetes mellitus, thyroid conditions, and metabolic disorders. Trained at the Clinic for Endocrinology in Skopje. Passionate about patient education and long-term disease management.',
    'Clinic for Endocrinology, Diabetes and Metabolic Disorders, Skopje',
    TRUE, 120.00, 55.00
),
(
    '22222222-2222-2222-2222-222222222224',
    'Orthopedics', 'Spine Surgery',
    'Dr. Michael Bennett',
    '/static/doctors/22222222-2222-2222-2222-222222222224.jpg',
    'Orthopaedic surgeon specialising in spine conditions including disc herniation, spinal stenosis, and scoliosis. 12 years of surgical and clinical experience. Completed a spine surgery fellowship at the University Hospital Zurich. Member of AO Spine.',
    'University Clinic for Traumatology and Orthopaedics, Skopje',
    TRUE, 140.00, 65.00
);

INSERT INTO languages (id) VALUES
    ('macedonian'),
    ('serbian'),
    ('english'),
    ('albanian');

INSERT INTO doctor_language (doctor_id, language_id) VALUES
    ('22222222-2222-2222-2222-222222222221', 'macedonian'),
    ('22222222-2222-2222-2222-222222222221', 'english'),
    ('22222222-2222-2222-2222-222222222222', 'macedonian'),
    ('22222222-2222-2222-2222-222222222222', 'serbian'),
    ('22222222-2222-2222-2222-222222222223', 'macedonian'),
    ('22222222-2222-2222-2222-222222222223', 'english'),
    ('22222222-2222-2222-2222-222222222223', 'albanian'),
    ('22222222-2222-2222-2222-222222222224', 'macedonian'),
    ('22222222-2222-2222-2222-222222222224', 'serbian'),
    ('22222222-2222-2222-2222-222222222224', 'english');

INSERT INTO consultations (
    id, patient_id, doctor_id, consultation_type, consultation_status,
    consultation_payment_status, price, urgent, need_translator,
    terms_accepted, email_sent_patient, signature_doctor_url,
    patient_name, patient_surname, patient_age, patient_phone, patient_email,
    patient_place_of_living, how_did_you_find_out, medical_problem,
    patient_diagnosis, allergies, medication, finished_at, created_at
) VALUES
-- Dr. Carter (Cardiology) — video
(
    '55555555-5555-5555-5555-555555555551',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222221',
    'video', 'pending', 'paid', 150.00, TRUE, FALSE, TRUE, FALSE, NULL,
    'Mary', 'Johnson', 45, '+38970123456', 'mary.johnson@example.com', 'Skopje', 'Google search',
    'Persistent chest tightness and shortness of breath for the past two weeks, worsening with physical activity. Episodes of palpitations lasting 5–10 minutes.',
    NULL, 'Penicillin', 'Salbutamol inhaler 100mcg as needed', NULL, NOW() - INTERVAL '2 days'
),
(
    '55555555-5555-5555-5555-555555555552',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222221',
    'video', 'finished', 'paid', 150.00, FALSE, FALSE, TRUE, TRUE,
    'https://storage.cybermedico.com/signature-carter-552.png',
    'Thomas', 'Walker', 58, '+38971234567', 'thomas.walker@example.com', 'Kumanovo', 'Referred by GP',
    'Hypertension poorly controlled despite current medication. Blood pressure readings averaging 158/96 mmHg at home over past four weeks.',
    'Resistant hypertension', 'Sulfa drugs', 'Amlodipine 10mg once daily, Ramipril 5mg once daily',
    NOW() - INTERVAL '73 days', NOW() - INTERVAL '27 days'
),
(
    '55555555-5555-5555-5555-555555555553',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222221',
    'video', 'scheduled', 'paid', 150.00, FALSE, FALSE, TRUE, FALSE, NULL,
    'Laura', 'Anderson', 52, '+38972345678', 'laura.anderson@example.com', 'Ohrid', 'Facebook ad',
    'Intermittent chest pain at rest, not clearly related to exertion. Associated with mild sweating and nausea on two occasions.',
    NULL, 'None', 'Aspirin 100mg once daily', NULL, NOW() - INTERVAL '22 days'
),
-- Dr. Wilson (Neurology) — video
(
    '55555555-5555-5555-5555-555555555554',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222',
    'video', 'pending', 'paid', 130.00, TRUE, FALSE, TRUE, FALSE, NULL,
    'Anna', 'Roberts', 28, '+38973456789', 'anna.roberts@example.com', 'Bitola', 'Referred by friend',
    'Sudden onset severe headache described as the worst headache of her life, accompanied by neck stiffness and mild fever since yesterday morning.',
    NULL, 'Codeine', 'None', NULL, NOW() - INTERVAL '18 days'
),
(
    '55555555-5555-5555-5555-555555555555',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222',
    'video', 'finished', 'paid', 130.00, FALSE, FALSE, TRUE, TRUE,
    'https://storage.cybermedico.com/signature-wilson-555.png',
    'Daniel', 'Hughes', 35, '+38974567890', 'daniel.hughes@example.com', 'Štip', 'Instagram',
    'Episodes of uncontrolled shaking of the right hand lasting 2–3 minutes, occurring twice weekly for two months. No loss of consciousness.',
    'Focal onset seizures — investigation ongoing', 'None', 'Levetiracetam 500mg twice daily (newly started)',
    NOW() - INTERVAL '66 days', NOW() - INTERVAL '15 days'
),
(
    '55555555-5555-5555-5555-555555555556',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222',
    'video', 'pending', 'pending', 130.00, FALSE, TRUE, TRUE, FALSE, NULL,
    'Lydia', 'Morgan', 44, '+38975678901', 'lydia.morgan@example.com', 'Gostivar', 'TV advertisement',
    'Chronic daily headaches for three months, predominantly frontal, with associated visual disturbances described as zigzag lines before the pain onset.',
    NULL, 'Ibuprofen (GI intolerance)', 'Sumatriptan 50mg as needed', NULL, NOW() - INTERVAL '15 days'
),
-- Dr. Mitchell (Endocrinology) — written
(
    '55555555-5555-5555-5555-555555555557',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222223',
    'written', 'pending', 'paid', 55.00, FALSE, FALSE, TRUE, FALSE, NULL,
    'Brian', 'Taylor', 54, '+38976789012', 'brian.taylor@example.com', 'Strumica', 'Google search',
    'Elevated fasting blood glucose readings at home ranging 7.2–8.9 mmol/L over the past month. No previous diagnosis of diabetes. Family history of type 2 diabetes on both parents.',
    NULL, 'Aspirin', 'Metformin 500mg twice daily (self-prescribed)', NULL, NOW() - INTERVAL '10 days'
),
(
    '55555555-5555-5555-5555-555555555558',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222223',
    'written', 'finished', 'paid', 55.00, FALSE, FALSE, TRUE, TRUE,
    'https://storage.cybermedico.com/signature-mitchell-558.png',
    'Megan', 'Clark', 42, '+38977890123', 'megan.clark@example.com', 'Prilep', 'Doctor recommendation',
    'Persistent fatigue, unexplained weight gain of 8kg over three months, cold intolerance, dry skin, and constipation. No known thyroid history.',
    'Hypothyroidism', 'None', 'Levothyroxine 50mcg once daily (newly initiated)',
    NOW() - INTERVAL '56 days', NOW() - INTERVAL '7 days'
),
(
    '55555555-5555-5555-5555-555555555559',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222223',
    'written', 'pending', 'paid', 55.00, FALSE, FALSE, TRUE, FALSE, NULL,
    'Helen', 'Turner', 38, '+38978901234', 'helen.turner@example.com', 'Tetovo', 'Referred by GP',
    'Recent lab results showing TSH 0.02 mIU/L with elevated free T4. Experiencing tremors, weight loss of 5kg in six weeks, and heart palpitations.',
    NULL, 'None', 'None', NULL, NOW() - INTERVAL '5 days'
),
-- Dr. Bennett (Orthopedics) — written
(
    '55555555-5555-5555-5555-55555555555a',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222224',
    'written', 'pending', 'paid', 65.00, FALSE, FALSE, TRUE, FALSE, NULL,
    'Steven', 'Harris', 32, '+38979012345', 'steven.harris@example.com', 'Veles', 'Facebook ad',
    'Recurring lower back pain radiating down the left leg to the calf, starting after lifting heavy furniture three months ago. Worsens with sitting for more than 30 minutes.',
    NULL, 'None', 'Ibuprofen 400mg three times daily, Diazepam 5mg at night', NULL, NOW() - INTERVAL '4 days'
),
(
    '55555555-5555-5555-5555-55555555555b',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222224',
    'written', 'finished', 'paid', 65.00, FALSE, FALSE, TRUE, TRUE,
    'https://storage.cybermedico.com/signature-bennett-55b.png',
    'Nicholas', 'Evans', 61, '+38970123456', 'nicholas.evans@example.com', 'Kavadarci', 'Google search',
    'Bilateral knee pain worsening over two years, crepitus on movement, morning stiffness lasting about 20 minutes. Difficulty climbing stairs.',
    'Bilateral knee osteoarthritis grade III', 'NSAIDs (gastric ulcer history)', 'Paracetamol 1g three times daily, glucosamine supplement',
    NOW() - INTERVAL '52 days', NOW() - INTERVAL '3 days'
),
(
    '55555555-5555-5555-5555-55555555555c',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222224',
    'written', 'pending', 'pending', 65.00, TRUE, FALSE, TRUE, FALSE, NULL,
    'Katherine', 'Lewis', 29, '+38971234567', 'katherine.lewis@example.com', 'Negotino', 'Instagram',
    'Acute onset neck pain and stiffness following a rear-end car accident four days ago. Pain radiates to the right shoulder. Mild headache since the incident.',
    NULL, 'None', 'Ibuprofen 600mg twice daily', NULL, NOW() - INTERVAL '1 day'
),
(
    '55555555-5555-5555-5555-55555555555d',
    '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222224',
    'written', 'not_attended', 'paid', 65.00, FALSE, TRUE, TRUE, FALSE, NULL,
    'Martin', 'Green', 40, '+38972345678', 'martin.green@example.com', 'Skopje', 'Google search',
    'Chronic lower back pain with intermittent numbness in the left leg. Pain worsens with prolonged sitting and standing.',
    NULL, 'None', 'Ibuprofen 600mg twice daily', NULL, NOW()
);

-- Video consultation records
INSERT INTO video_consultations (consultation_id, preferred_time, scheduled_at) VALUES
    ('55555555-5555-5555-5555-555555555551', 'morning', NULL),
    ('55555555-5555-5555-5555-555555555552', 'afternoon', NULL),
    ('55555555-5555-5555-5555-555555555553', 'morning', NOW() + '1 day'),
    ('55555555-5555-5555-5555-555555555554', 'evening', NULL),
    ('55555555-5555-5555-5555-555555555555', 'morning', NULL),
    ('55555555-5555-5555-5555-555555555556', 'afternoon', NULL);

-- Written consultation records
INSERT INTO written_consultations (consultation_id) VALUES
    ('55555555-5555-5555-5555-555555555557'),
    ('55555555-5555-5555-5555-555555555558'),
    ('55555555-5555-5555-5555-555555555559'),
    ('55555555-5555-5555-5555-55555555555a'),
    ('55555555-5555-5555-5555-55555555555b'),
    ('55555555-5555-5555-5555-55555555555c');

-- Not attended: its 72 hours ran out (the trigger sets expires_at from created_at)
INSERT INTO written_consultations (consultation_id, created_at) VALUES
    ('55555555-5555-5555-5555-55555555555d', NOW() - INTERVAL '4 days');

INSERT INTO doctor_contacts (doctor_id, phone_number, meeting_link) VALUES
('22222222-2222-2222-2222-222222222221', '049 834 233', 'https://meet.cybermedico.net/dr-carter'),
('22222222-2222-2222-2222-222222222222', '948 394 123','https://meet.cybermedico.net/dr-wilson'),
('22222222-2222-2222-2222-222222222223', '843 234 564', 'https://meet.cybermedico.net/dr-mitchell'),
('22222222-2222-2222-2222-222222222224', '345 234 567', 'https://meet.cybermedico.net/dr-bennett');

INSERT INTO consultation_attachments (consultation_id, s3_key, original_filename, content_type, size_bytes) VALUES
-- Mary Johnson (551) — ECG report
('55555555-5555-5555-5555-555555555551', 'consultations/55555555-5555-5555-5555-555555555551/ecg-report-mary.pdf',       'ecg-report-mary.pdf',       'application/pdf', 184320),
-- Thomas Walker (552) — BP log + echocardiogram
('55555555-5555-5555-5555-555555555552', 'consultations/55555555-5555-5555-5555-555555555552/bp-log-thomas.pdf',             'bp-log-thomas.pdf',             'application/pdf', 102400),
('55555555-5555-5555-5555-555555555552', 'consultations/55555555-5555-5555-5555-555555555552/echocardiogram-thomas.pdf',     'echocardiogram-thomas.pdf',     'application/pdf', 512000),
-- Laura Anderson (553) — stress test
('55555555-5555-5555-5555-555555555553', 'consultations/55555555-5555-5555-5555-555555555553/stress-test-laura.pdf',       'stress-test-laura.pdf',       'application/pdf', 245760),
-- Anna Roberts (554) — CT head
('55555555-5555-5555-5555-555555555554', 'consultations/55555555-5555-5555-5555-555555555554/ct-head-anna.pdf',             'ct-head-anna.pdf',             'application/pdf', 819200),
-- Daniel Hughes (555) — EEG + MRI brain
('55555555-5555-5555-5555-555555555555', 'consultations/55555555-5555-5555-5555-555555555555/eeg-daniel.pdf',               'eeg-daniel.pdf',               'application/pdf', 163840),
('55555555-5555-5555-5555-555555555555', 'consultations/55555555-5555-5555-5555-555555555555/mri-brain-daniel.pdf',         'mri-brain-daniel.pdf',         'application/pdf', 1048576),
-- Lydia Morgan (556) — no attachments
-- Brian Taylor (557) — glucose log + bloodwork
('55555555-5555-5555-5555-555555555557', 'consultations/55555555-5555-5555-5555-555555555557/glucose-log-brian.pdf',       'glucose-log-brian.pdf',       'application/pdf', 73728),
('55555555-5555-5555-5555-555555555557', 'consultations/55555555-5555-5555-5555-555555555557/bloodwork-brian.pdf',         'bloodwork-brian.pdf',         'application/pdf', 204800),
-- Megan Clark (558) — thyroid panel + TSH
('55555555-5555-5555-5555-555555555558', 'consultations/55555555-5555-5555-5555-555555555558/thyroid-panel-megan.pdf',      'thyroid-panel-megan.pdf',      'application/pdf', 153600),
('55555555-5555-5555-5555-555555555558', 'consultations/55555555-5555-5555-5555-555555555558/tsh-megan.pdf',                'tsh-megan.pdf',                'application/pdf', 92160),
-- Helen Turner (559) — thyroid panel + ECG
('55555555-5555-5555-5555-555555555559', 'consultations/55555555-5555-5555-5555-555555555559/thyroid-panel-helen.pdf',     'thyroid-panel-helen.pdf',     'application/pdf', 153600),
('55555555-5555-5555-5555-555555555559', 'consultations/55555555-5555-5555-5555-555555555559/ecg-helen.pdf',               'ecg-helen.pdf',               'application/pdf', 184320),
-- Steven Harris (55a) — lumbar MRI + X-ray
('55555555-5555-5555-5555-55555555555a', 'consultations/55555555-5555-5555-5555-55555555555a/mri-lumbar-steven.pdf',       'mri-lumbar-steven.pdf',       'application/pdf', 1048576),
('55555555-5555-5555-5555-55555555555a', 'consultations/55555555-5555-5555-5555-55555555555a/xray-lumbar-steven.jpg',      'xray-lumbar-steven.jpg',      'image/jpeg',      368640),
-- Nicholas Evans (55b) — knee X-ray
('55555555-5555-5555-5555-55555555555b', 'consultations/55555555-5555-5555-5555-55555555555b/xray-knees-nicholas.pdf',       'xray-knees-nicholas.pdf',       'application/pdf', 286720),
-- Katherine Lewis (55c) — cervical X-ray + accident report
('55555555-5555-5555-5555-55555555555c', 'consultations/55555555-5555-5555-5555-55555555555c/xray-cervical-katherine.pdf',  'xray-cervical-katherine.pdf',  'application/pdf', 245760),
('55555555-5555-5555-5555-55555555555c', 'consultations/55555555-5555-5555-5555-55555555555c/accident-report-katherine.pdf','accident-report-katherine.pdf','application/pdf', 122880),
-- Martin Green (55d) — lumbar MRI
('55555555-5555-5555-5555-55555555555d', 'consultations/55555555-5555-5555-5555-55555555555d/mri-lumbar-martin.pdf',       'mri-lumbar-martin.pdf',       'application/pdf', 1048576);

INSERT INTO doctor_financials (doctor_id, bank_account, provision) VALUES
('22222222-2222-2222-2222-222222222221', 'MK123456789012345', 20),
('22222222-2222-2222-2222-222222222222', 'MK987654321098765', 20),
('22222222-2222-2222-2222-222222222223', 'MK111122223333444', 20),
('22222222-2222-2222-2222-222222222224', 'MK444333221111000', 20);

-- Dashboard widgets are shared per role. Each widget stores its SQL query (along
-- with grid layout and styling) in the JSONB config; the widget endpoint runs the
-- query on demand via Datastar, injecting the current user's id as the @user_id
-- named parameter. Admin widgets are aggregate and ignore @user_id; doctor
-- widgets reference it to scope data to the signed-in doctor (doctor_id == users.id).
INSERT INTO dashboard_widgets (id, role, title, type, layout, config) VALUES
(
    '66666666-6666-6666-6666-666666666661',
    'admin',
    'Total Consultations',
    'text',
    '{"x":0,"y":0,"w":3,"h":1}',
    jsonb_build_object(
        'text_font_size', 28,
        'sub_font_size', 14,
        'query', $q$
            SELECT
                (SELECT COUNT(*) FROM consultations)::text AS text,
                'Total Consultations' AS subtext,
                COALESCE((
                    SELECT json_agg(json_build_object('date', to_char(day, 'DD/MM/YYYY'), 'value', cnt) ORDER BY day)
                    FROM (
                        SELECT DATE_TRUNC('day', created_at) AS day, COUNT(*) AS cnt
                        FROM consultations
                        WHERE created_at >= NOW() - INTERVAL '30 days'
                        GROUP BY day
                    ) s
                )::text, '[]') AS data
        $q$
    )
),
(
    '66666666-6666-6666-6666-666666666662',
    'admin',
    'Consultation Types',
    'pie',
    '{"x":3,"y":0,"w":1,"h":1}',
    jsonb_build_object(
        'series_name', 'Consultations',
        'query', $q$
            SELECT consultation_type AS label, COUNT(*) AS value
            FROM consultations
            GROUP BY consultation_type
            ORDER BY consultation_type
        $q$
    )
),
-- One shared definition for every doctor; @user_id scopes it to the signed-in one.
(
    '66666666-6666-6666-6666-666666666663',
    'doctor',
    'My Consultations',
    'text',
    '{"x":0,"y":0,"w":3,"h":1}',
    jsonb_build_object(
        'text_font_size', 28,
        'sub_font_size', 14,
        'query', $q$
            SELECT
                (SELECT COUNT(*) FROM consultations WHERE doctor_id = @user_id)::text AS text,
                'My Consultations' AS subtext,
                COALESCE((
                    SELECT json_agg(json_build_object('date', to_char(day, 'DD/MM/YYYY'), 'value', cnt) ORDER BY day)
                    FROM (
                        SELECT DATE_TRUNC('day', created_at) AS day, COUNT(*) AS cnt
                        FROM consultations
                        WHERE doctor_id = @user_id
                            AND created_at >= NOW() - INTERVAL '30 days'
                        GROUP BY day
                    ) s
                )::text, '[]') AS data
        $q$
    )
),
(
    '66666666-6666-6666-6666-666666666664',
    'doctor',
    'My Consultation Types',
    'pie',
    '{"x":3,"y":0,"w":1,"h":1}',
    jsonb_build_object(
        'series_name', 'Consultations',
        'query', $q$
            SELECT consultation_type AS label, COUNT(*) AS value
            FROM consultations
            WHERE doctor_id = @user_id
            GROUP BY consultation_type
            ORDER BY consultation_type
        $q$
    )
),
-- Top 4 doctors by all-time consultation count (admin overview). The inner query
-- ranks doctors by volume and keeps the busiest 4; the outer query re-orders them
-- ascending so the busiest lands at the top of the horizontal bar chart.
(
    '66666666-6666-6666-6666-666666666665',
    'admin',
    'Top Doctors by Consultations',
    'bar',
    '{"x":0,"y":2.5,"w":3,"h":1}',
    jsonb_build_object(
        'series_name', 'Consultations',
        'query', $q$
            SELECT label, value
            FROM (
                SELECT dp.display_name AS label, COUNT(c.id) AS value
                FROM doctor_profile dp
                LEFT JOIN consultations c ON c.doctor_id = dp.id
                GROUP BY dp.display_name
                ORDER BY value DESC
                LIMIT 4
            ) top_doctors
            ORDER BY value ASC
        $q$
    )
),
-- All-time breakdown of the signed-in doctor's own consultations by status.
(
    '66666666-6666-6666-6666-666666666666',
    'doctor',
    'My Consultations by Status',
    'bar',
    '{"x":0,"y":2.5,"w":3,"h":2}',
    jsonb_build_object(
        'series_name', 'Consultations',
        'query', $q$
            SELECT consultation_status AS label, COUNT(*) AS value
            FROM consultations
            WHERE doctor_id = @user_id
                AND consultation_status != 'not_attended'
            GROUP BY consultation_status
            ORDER BY value ASC
        $q$
    )
);