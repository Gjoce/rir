package policies_test

import data.policies

doctor_management_operations := [
	"doctors_form",
	"doctors_add_form",
	"doctor_page",
	"edit_doctor_form",
	"edit_doctor_submit",
	"delete_doctor",
	"change_provision",
	"change_doctor_availability",
]

test_admin_manages_doctors if {
	every operation in doctor_management_operations {
		policies.allow with input as request(operation, admin, no_target)
	}
}

test_doctor_cannot_manage_doctors if {
	every operation in doctor_management_operations {
		not policies.allow with input as request(operation, doctor, no_target)
	}
}

test_anonymous_cannot_manage_doctors if {
	every operation in doctor_management_operations {
		not policies.allow with input as request(operation, anonymous, no_target)
	}
}

test_admin_changes_doctor_availability if {
	policies.allow with input as request("change_doctor_availability", admin, no_target)
}

test_doctor_cannot_change_doctor_availability if {
	not policies.allow with input as request("change_doctor_availability", doctor, no_target)
}

test_anonymous_cannot_change_doctor_availability if {
	not policies.allow with input as request("change_doctor_availability", anonymous, no_target)
}

test_unknown_role_cannot_change_doctor_availability if {
	not policies.allow with input as request("change_doctor_availability", {"UserID": "x", "Role": "nurse"}, no_target)
}
