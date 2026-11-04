# A consultation's details need a login
GET {{host}}/consultations/55555555-5555-5555-5555-555555555551
HTTP 303
[Asserts]
header "Location" == "/auth/login"
body not contains "Persistent chest tightness"

# Without a login, a malformed id goes to the login page as well
GET {{host}}/consultations/not-a-uuid
HTTP 303
[Asserts]
header "Location" == "/auth/login"
