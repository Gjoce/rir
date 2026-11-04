# The landing page lists the available doctors with their details
GET {{host}}/
HTTP 200
[Asserts]
body contains "Dr. Emily Carter"
body contains "Dr. James Wilson"
body contains "Dr. Sarah Mitchell"
body contains "Dr. Michael Bennett"
body contains "/static/doctors/22222222-2222-2222-2222-222222222221.jpg"
body contains "University Clinic for Cardiology, Skopje"
body contains "english, macedonian"
body contains "70.00 €"
body contains "150.00 €"

# The filter offers the specializations
GET {{host}}/
HTTP 200
[Asserts]
body contains "All specializations"
body contains "<option value=\"Cardiology\""
body contains "<option value=\"Neurology\""

# Filtering by specialization shows only those doctors
GET {{host}}/
Datastar-Request: true
[Query]
datastar: {"specialization":"Cardiology"}
HTTP 200
[Asserts]
header "Content-Type" contains "text/event-stream"
body contains "Dr. Emily Carter"
body not contains "Dr. James Wilson"
body not contains "Dr. Michael Bennett"

# Clearing the filter shows everyone again
GET {{host}}/
Datastar-Request: true
[Query]
datastar: {"specialization":""}
HTTP 200
[Asserts]
body contains "Dr. Emily Carter"
body contains "Dr. James Wilson"

# Malformed filter input is refused
GET {{host}}/
[Query]
datastar: {bad
HTTP 400

# A specialization without doctors shows a message and a link to the full list
GET {{host}}/
Datastar-Request: true
[Query]
datastar: {"specialization":"Dermatology"}
HTTP 200
[Asserts]
body contains "No doctors are currently available."
body contains "href=\"/\""
body not contains "Dr. Emily Carter"
