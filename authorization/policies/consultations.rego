package policies

allow if {
	input.operation_id == "consultations"
	input.caller.Role == "admin"
}

allow if {
	input.operation_id == "consultations"
	input.caller.Role == "doctor"
}

allow if {
	input.operation_id == "consultation"
	input.caller.Role == "admin"
}

allow if {
	input.operation_id == "consultation"
	input.caller.Role == "doctor"
	input.caller.UserID == input.target.UserID
	input.target.ConsultationStatus != "not_attended"
}

allow if {
	input.operation_id == "schedule_consultation"
	input.caller.Role == "doctor"
	input.caller.UserID == input.target.UserID
	input.target.ConsultationStatus == "pending"
	input.target.ConsultationType == "video"
}

allow if {
	input.operation_id == "answer_consultation"
	input.caller.Role == "doctor"
	input.caller.UserID == input.target.UserID
	input.target.ConsultationStatus == "pending"
	input.target.ConsultationType == "written"
}

allow if {
	input.operation_id == "answer_consultation"
	input.caller.Role == "doctor"
	input.caller.UserID == input.target.UserID
	input.target.ConsultationStatus == "scheduled"
	input.target.ConsultationType == "video"
	input.target.ScheduledAtPast == true
}

allow if {
	input.operation_id == "view_not_attended"
	input.caller.Role == "admin"
}

allow if {
	input.operation_id == "add_consultation"
	input.caller.Role == "admin"
}

allow if {
	input.operation_id == "join_video_consultation"
	input.caller.Role == "doctor"
	input.caller.UserID == input.target.UserID
	input.target.ConsultationStatus == "scheduled"
	input.target.ConsultationType == "video"
	input.target.WithinJoinWindow == true
}

allow if {
	input.operation_id == "join_video_consultation_patient"
	input.target.ConsultationStatus == "scheduled"
	input.target.ConsultationType == "video"
	input.target.WithinJoinWindow == true
}
