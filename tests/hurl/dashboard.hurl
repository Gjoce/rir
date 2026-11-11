# The dashboard needs a login
GET {{host}}/dashboard
HTTP 303
[Asserts]
header "Location" == "/auth/login"

# So do its chart widgets, which run queries over all consultations
GET {{host}}/widgets/66666666-6666-6666-6666-666666666661
HTTP 303
[Asserts]
header "Location" == "/auth/login"

GET {{host}}/widgets/66666666-6666-6666-6666-666666666661
Datastar-Request: true
HTTP 303
[Asserts]
header "Location" == "/auth/login"
