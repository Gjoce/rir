package policies_test

import data.policies

test_admin_opens_river_ui if {
	policies.allow with input as request("access_riverui", admin, no_target)
}

test_doctor_cannot_open_river_ui if {
	not policies.allow with input as request("access_riverui", doctor, no_target)
}

test_anonymous_cannot_open_river_ui if {
	not policies.allow with input as request("access_riverui", anonymous, no_target)
}
