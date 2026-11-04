package policies_test

admin := {"UserID": "admin-id", "Role": "admin"}

doctor := {"UserID": "own-doctor-id", "Role": "doctor"}

anonymous := {"UserID": "", "Role": ""}

no_target := {}

request(operation, caller, target) := {
	"operation_id": operation,
	"caller": caller,
	"target": target,
}

own(type, status) := {
	"UserID": "own-doctor-id",
	"ConsultationType": type,
	"ConsultationStatus": status,
	"ScheduledAtPast": false,
	"WithinJoinWindow": false,
}

other_doctors(type, status) := object.union(own(type, status), {"UserID": "other-doctor-id"})

own_in_join_window(type, status) := object.union(own(type, status), {"WithinJoinWindow": true})

own_after_appointment(type, status) := object.union(own(type, status), {"ScheduledAtPast": true})
