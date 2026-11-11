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
	not input.target.OpinionDeadlinePassed
}

allow if {
	input.operation_id == "view_not_attended"
	input.caller.Role == "admin"
}

allow if {
	input.operation_id == "add_consultation"
	input.caller.Role == "admin"
}

# The call is the assigned doctor's to enter, whether or not it is open right now:
# the join button and the doctor's call page are shown for it.
allow if {
	input.operation_id == "view_video_call"
	own_scheduled_video_call
}

allow if {
	input.operation_id == "join_video_consultation"
	own_scheduled_video_call
	input.target.WithinJoinWindow == true
}

own_scheduled_video_call if {
	input.caller.Role == "doctor"
	input.caller.UserID == input.target.UserID
	input.target.ConsultationStatus == "scheduled"
	input.target.ConsultationType == "video"
}

allow if {
	input.operation_id == "join_video_consultation_patient"
	input.target.ConsultationStatus == "scheduled"
	input.target.ConsultationType == "video"
	input.target.WithinJoinWindow == true
}
