# Public page uses the public layout
GET {{host}}/
HTTP 200
[Asserts]
body contains "CyberMedico"
body not contains "Panel"

# Choosing a theme stores it and returns to the same page
GET {{host}}/themes/dark
Referer: {{host}}/dashboard
HTTP 303
[Asserts]
header "Location" == "/dashboard"
cookie "cybermedico_theme" == "dark"

# The chosen theme is applied to the page
GET {{host}}/
HTTP 200
[Asserts]
body contains "data-theme=\"dark\""

# A Referer from another site can't redirect away from the app
GET {{host}}/themes/auto
Referer: https://example.com/phishing
HTTP 303
[Asserts]
header "Location" == "/phishing"

# Unknown theme is rejected
GET {{host}}/themes/neon
HTTP 400

# Unknown pages use the error layout
GET {{host}}/does-not-exist
HTTP 404
[Asserts]
body contains "Page Not Found"

# Static files are served
GET {{host}}/static/logo.svg
HTTP 200
