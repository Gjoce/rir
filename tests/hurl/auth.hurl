# Protected pages send visitors without a session to the login page
GET {{host}}/dashboard
HTTP 303
[Asserts]
header "Location" == "/auth/login"

# Login page
GET {{host}}/auth/login
HTTP 200
[Asserts]
body contains "Welcome back"
body not contains "logout-modal"

# Empty email
POST {{host}}/auth/login
[Form]
email:
password: x
HTTP 200
[Asserts]
body contains "Email is required"

# Wrong password and unknown email get the same generic message
POST {{host}}/auth/login
[Form]
email: dr.carter@cybermedico.com
password: wrong-password
HTTP 200
[Asserts]
body contains "Invalid email or password"
body contains "dr.carter@cybermedico.com"

POST {{host}}/auth/login
[Form]
email: nobody@example.com
password: wrong-password
HTTP 200
[Asserts]
body contains "Invalid email or password"

# Accounts without a role can't log in, even with the right password
POST {{host}}/auth/login
[Form]
email: patient@example.com
password: cybermedico-dev
HTTP 200
[Asserts]
body contains "Invalid email or password"

# Failed logins don't open protected pages
GET {{host}}/dashboard
HTTP 303

# The code page needs a correct password first
GET {{host}}/auth/2fa
HTTP 303
[Asserts]
header "Location" == "/auth/login"

# Correct credentials lead to the code page, not to a full session
POST {{host}}/auth/login
[Form]
email: dr.carter@cybermedico.com
password: cybermedico-dev
HTTP 303
[Asserts]
header "Location" == "/auth/2fa"
cookie "cybermedico_session[HttpOnly]" exists
cookie "cybermedico_session[Secure]" exists

GET {{host}}/dashboard
HTTP 303
[Asserts]
header "Location" == "/auth/login"

GET {{host}}/auth/2fa
HTTP 200
[Asserts]
body contains "6-digit code"

# Wrong code
POST {{host}}/auth/2fa
[Form]
totp_code: 000000
HTTP 400
[Asserts]
body contains "Invalid code"

GET {{host}}/dashboard
HTTP 303

# An account that already has a secret can't replace it
GET {{host}}/auth/2fa/setup
HTTP 303
[Asserts]
header "Location" == "/auth/2fa"
