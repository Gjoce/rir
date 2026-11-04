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
