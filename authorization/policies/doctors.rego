package policies

allow if {
    input.operation_id == "doctors_form"
    input.caller.Role == "admin"
}

allow if {
    input.operation_id == "doctors_add_form"
    input.caller.Role == "admin"
}

allow if {
    input.operation_id == "doctor_page"
    input.caller.Role == "admin"
}

allow if {
    input.operation_id in ["edit_doctor_form", "edit_doctor_submit"]
    input.caller.Role == "admin"
}

allow if {
    input.operation_id == "delete_doctor"
    input.caller.Role == "admin"
}

allow if {
    input.operation_id == "change_provision"
    input.caller.Role == "admin"
}
