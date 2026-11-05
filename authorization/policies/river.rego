package policies

allow if {
	input.operation_id == "access_riverui"
	input.caller.Role == "admin"
}
