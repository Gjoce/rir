# The answer form needs a login
GET {{host}}/consultations/55555555-5555-5555-5555-555555555557/answer
HTTP 303
[Asserts]
header "Location" == "/auth/login"

# So does sending an answer
POST {{host}}/consultations/55555555-5555-5555-5555-555555555557/answer
Content-Type: application/json
{"answer":"<p>Not logged in</p>"}
HTTP 303
[Asserts]
header "Location" == "/auth/login"
