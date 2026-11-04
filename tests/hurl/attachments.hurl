# The form offers file upload
GET {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
HTTP 200
[Asserts]
body contains "name=\"attachments\""
body contains "accept=\".pdf,.jpg,.jpeg,.png\""

# PDF, JPG and PNG are accepted
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
[Multipart]
patient_name: Attachment
patient_surname: Check
patient_age: 40
patient_email: hurl.attachments@example.com
patient_phone: 041000000
medical_problem: Results attached
terms_accepted: true
attachments: file,fixtures/report.pdf; application/pdf
attachments: file,fixtures/photo.jpg; image/jpeg
attachments: file,fixtures/scan.png; image/png
HTTP 303
[Asserts]
header "Location" == "/order-submitted"

# The content decides, not the name or the type the browser reports
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
[Multipart]
patient_name: Attachment
patient_surname: Check
patient_age: 40
patient_email: hurl.refused@example.com
patient_phone: 041000000
medical_problem: Results attached
terms_accepted: true
attachments: file,fixtures/fake.pdf; application/pdf
HTTP 200
[Asserts]
body contains "must be a PDF, JPG or PNG file"
body contains "value=\"Attachment\""
body contains "Results attached"

# More than 5 files, and the form keeps what was entered
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
[Multipart]
patient_name: Attachment
patient_surname: Check
patient_age: 40
patient_email: hurl.refused@example.com
patient_phone: 041000000
medical_problem: Results attached
terms_accepted: true
attachments: file,fixtures/report.pdf; application/pdf
attachments: file,fixtures/report.pdf; application/pdf
attachments: file,fixtures/report.pdf; application/pdf
attachments: file,fixtures/report.pdf; application/pdf
attachments: file,fixtures/report.pdf; application/pdf
attachments: file,fixtures/report.pdf; application/pdf
HTTP 200
[Asserts]
body contains "You can attach at most 5 files"
body contains "value=\"Attachment\""

# Files with a mistake elsewhere in the form: the patient is told to choose them again
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
[Multipart]
patient_name: Attachment
attachments: file,fixtures/report.pdf; application/pdf
HTTP 200
[Asserts]
body contains "Your files were not saved"
body contains "Surname is required"
