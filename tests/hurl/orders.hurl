# The profile button opens the form with the doctor and the price
GET {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
HTTP 200
[Asserts]
body contains "Dr. Emily Carter"
body contains "70.00 €"
body contains "name=\"patient_name\""
body contains "name=\"terms_accepted\""
body contains "enctype=\"multipart/form-data\""

# An unknown doctor can't be ordered from
GET {{host}}/specialists/00000000-0000-0000-0000-000000000000/written
HTTP 404

# An empty form shows every required field and nothing is saved
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
[Multipart]
patient_name:
HTTP 200
[Asserts]
body contains "Name is required"
body contains "Surname is required"
body contains "Age is required"
body contains "Email is required"
body contains "Phone is required"
body contains "Description of the problem is required"
body contains "You have to accept the terms of use"

# Wrong values show an error and keep what was entered, safely escaped
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
[Multipart]
patient_name: <script>alert(1)</script>
patient_surname: Walker
patient_age: abc
patient_email: not-an-email
patient_phone: 041000000
medical_problem: Chest pain
terms_accepted: true
HTTP 200
[Asserts]
body contains "Age must be a number between 0 and 120"
body contains "Enter a valid email address"
body contains "&lt;script&gt;alert(1)&lt;/script&gt;"
body not contains "<script>alert(1)</script>"
body contains "value=\"Walker\""
body contains "Chest pain"
body not contains "Name is required"

# Age out of range
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
[Multipart]
patient_name: Thomas
patient_surname: Walker
patient_age: 121
patient_email: hurl.check@example.com
patient_phone: 041000000
medical_problem: Chest pain
terms_accepted: true
HTTP 200
[Asserts]
body contains "Age must be a number between 0 and 120"

# Terms not accepted
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
[Multipart]
patient_name: Thomas
patient_surname: Walker
patient_age: 40
patient_email: hurl.check@example.com
patient_phone: 041000000
medical_problem: Chest pain
HTTP 200
[Asserts]
body contains "You have to accept the terms of use"

# A valid form creates the consultation
POST {{host}}/specialists/22222222-2222-2222-2222-222222222221/written
[Multipart]
patient_name: Thomas
patient_surname: Walker
patient_age: 40
patient_email: hurl.check@example.com
patient_phone: 041000000
medical_problem: Chest pain when climbing stairs
need_translator: true
terms_accepted: true
HTTP 303
[Asserts]
header "Location" == "/order-submitted"

GET {{host}}/order-submitted
HTTP 200
[Asserts]
body contains "Your consultation has been submitted"
