# The patient page needs the link token
GET {{host}}/video/55555555-5555-5555-5555-555555555553
HTTP 400

# A token that isn't ours is refused
GET {{host}}/video/55555555-5555-5555-5555-555555555553?t=not.valid
HTTP 403

# The doctor page needs a login
GET {{host}}/consultations/55555555-5555-5555-5555-555555555553/video
HTTP 303

# TURN credentials only for the two sides of the call: nobody logged in and no token
GET {{host}}/api/video/55555555-5555-5555-5555-555555555553/ice
HTTP 401
[Asserts]
jsonpath "$.error" == "unauthorized"
jsonpath "$.iceServers" not exists

# A token that isn't ours
GET {{host}}/api/video/55555555-5555-5555-5555-555555555553/ice?t=not.valid
HTTP 403
[Asserts]
jsonpath "$.iceServers" not exists
