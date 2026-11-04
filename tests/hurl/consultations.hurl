# The consultation list needs a login
GET {{host}}/consultations
HTTP 303
[Asserts]
header "Location" == "/auth/login"

# A list request with a filter does too
GET {{host}}/consultations
Datastar-Request: true
[Query]
datastar: {"type":"not_attended"}
HTTP 303
[Asserts]
header "Location" == "/auth/login"
