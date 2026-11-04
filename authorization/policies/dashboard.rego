package policies

allow if {
    input.operation_id == "view_dashboard"
    input.caller.Role == "admin"
}

allow if {
    input.operation_id == "view_dashboard"
    input.caller.Role == "doctor"
}
