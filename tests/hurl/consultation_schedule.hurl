# The scheduling form needs a login
GET {{host}}/consultations/55555555-5555-5555-5555-555555555551/schedule
HTTP 303
[Asserts]
header "Location" == "/auth/login"

# So does setting an appointment
POST {{host}}/consultations/55555555-5555-5555-5555-555555555551/schedule
[Form]
date: 2030-01-01T10:00:00Z
HTTP 303
[Asserts]
header "Location" == "/auth/login"
