# The video form shows the video price and the extra fields
GET {{host}}/specialists/22222222-2222-2222-2222-222222222221/video
HTTP 200
[Asserts]
body contains "Video consultation"
body contains "150.00 €"
body contains "name=\"preferred_time\""
body contains "name=\"urgent\""
body contains "action=\"/specialists/22222222-2222-2222-2222-222222222221/video\""

# The written form doesn't have them
GET {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
HTTP 200
[Asserts]
body not contains "name=\"preferred_time\""
body not contains "name=\"urgent\""
body contains "action=\"/specialists/22222222-2222-2222-2222-222222222221/written\""

# An unknown doctor
GET {{host}}/specialists/00000000-0000-0000-0000-000000000000/video
HTTP 404

# The part of the day is required, and the other fields are checked as usual
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/video
[Multipart]
patient_name: Video
HTTP 200
[Asserts]
body contains "Choose the part of the day that suits you"
body contains "Surname is required"
body contains "value=\"Video\""

# A value that isn't one of the choices
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/video
[Multipart]
patient_name: Video
patient_surname: Check
patient_age: 40
patient_email: hurl.video.refused@example.com
patient_phone: 041000000
medical_problem: Headaches
preferred_time: midnight
terms_accepted: true
HTTP 200
[Asserts]
body contains "Choose the part of the day that suits you"

# The choice and the urgent flag stay selected when the form comes back
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/video
[Multipart]
patient_name: Video
preferred_time: evening
urgent: true
HTTP 200
[Asserts]
xpath "string(//select[@name='preferred_time']/option[@selected]/@value)" == "evening"
xpath "count(//input[@name='urgent'][@checked])" == 1

# A normal video order
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/video
[Multipart]
patient_name: Video
patient_surname: Check
patient_age: 40
patient_email: hurl.video@example.com
patient_phone: 041000000
medical_problem: Headaches
preferred_time: morning
terms_accepted: true
attachments: file,fixtures/report.pdf; application/pdf
HTTP 303
[Asserts]
header "Location" == "/order-submitted"

# An urgent video order
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/video
[Multipart]
patient_name: Video
patient_surname: Urgent
patient_age: 40
patient_email: hurl.video.urgent@example.com
patient_phone: 041000000
medical_problem: Severe headaches
preferred_time: evening
urgent: true
terms_accepted: true
HTTP 303

# A written order can't be made urgent
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
[Multipart]
patient_name: Written
patient_surname: Urgent
patient_age: 40
patient_email: hurl.written.urgent@example.com
patient_phone: 041000000
medical_problem: Headaches
preferred_time: evening
urgent: true
terms_accepted: true
HTTP 303
