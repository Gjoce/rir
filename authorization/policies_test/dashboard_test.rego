package policies_test

import data.policies

test_admin_sees_dashboard if {
	policies.allow with input as request("view_dashboard", admin, no_target)
}

test_doctor_sees_dashboard if {
	policies.allow with input as request("view_dashboard", doctor, no_target)
}

test_anonymous_cannot_see_dashboard if {
	not policies.allow with input as request("view_dashboard", anonymous, no_target)
}

test_unknown_role_cannot_see_dashboard if {
	not policies.allow with input as request("view_dashboard", {"UserID": "x", "Role": "nurse"}, no_target)
}

test_unknown_operation_is_refused if {
	not policies.allow with input as request("does_not_exist", admin, no_target)
}
