# The doctor list links to the profile
GET {{host}}/
HTTP 200
[Asserts]
body contains "href=\"/specialists/22222222-2222-2222-2222-222222222221\""

# The profile shows the doctor's details and both ways to order
GET {{host}}/specialists/22222222-2222-2222-2222-222222222221
HTTP 200
[Asserts]
body contains "Dr. Emily Carter"
body contains "Cardiology"
body contains "Interventional Cardiology"
body contains "Board-certified cardiologist"
body contains "University Clinic for Cardiology, Skopje"
body contains "english, macedonian"
body contains "70.00 €"
body contains "150.00 €"
body contains "href=\"/specialists/22222222-2222-2222-2222-222222222221/written\""
body contains "href=\"/specialists/22222222-2222-2222-2222-222222222221/video\""
body not contains "Panel"

# Private details stay private
GET {{host}}/specialists/22222222-2222-2222-2222-222222222221
HTTP 200
[Asserts]
body not contains "049 834 233"
body not contains "MK123456789012345"
body not contains "dr.carter@cybermedico.com"

# An unknown doctor
GET {{host}}/specialists/00000000-0000-0000-0000-000000000000
HTTP 404
[Asserts]
body contains "This doctor is not available."

# An account without a doctor profile, like the administrator
GET {{host}}/specialists/11111111-1111-1111-1111-111111111111
HTTP 404

# A malformed id
GET {{host}}/specialists/not-a-uuid
HTTP 400
[Asserts]
body contains "This request is not valid."

# A login request with a missing field
POST {{host}}/auth/login
[Form]
email: dr.carter@cybermedico.com
HTTP 400
