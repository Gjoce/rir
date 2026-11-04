package policies_test

import data.policies

# --- consultation list ---

test_admin_lists_consultations if {
	policies.allow with input as request("consultations", admin, no_target)
}

test_doctor_lists_consultations if {
	policies.allow with input as request("consultations", doctor, no_target)
}

test_anonymous_cannot_list_consultations if {
	not policies.allow with input as request("consultations", anonymous, no_target)
}

# --- US-04: a doctor only reaches their own consultations ---

test_doctor_opens_own_consultation if {
	policies.allow with input as request("consultation", doctor, own("written", "pending"))
}

test_doctor_cannot_open_other_doctors_consultation if {
	not policies.allow with input as request("consultation", doctor, other_doctors("written", "pending"))
}

test_admin_opens_any_consultation if {
	policies.allow with input as request("consultation", admin, other_doctors("written", "pending"))
}

test_anonymous_cannot_open_consultation if {
	not policies.allow with input as request("consultation", anonymous, own("written", "pending"))
}

# --- US-04: answering a written consultation ---

test_doctor_answers_own_pending_written if {
	policies.allow with input as request("answer_consultation", doctor, own("written", "pending"))
}

test_doctor_cannot_answer_other_doctors_written if {
	not policies.allow with input as request("answer_consultation", doctor, other_doctors("written", "pending"))
}

test_doctor_cannot_answer_finished_written if {
	not policies.allow with input as request("answer_consultation", doctor, own("written", "finished"))
}

test_doctor_cannot_answer_pending_video if {
	not policies.allow with input as request("answer_consultation", doctor, own("video", "pending"))
}

test_admin_cannot_answer if {
	not policies.allow with input as request("answer_consultation", admin, own("written", "pending"))
}

# --- US-05: scheduling a video call ---

test_doctor_schedules_own_pending_video if {
	policies.allow with input as request("schedule_consultation", doctor, own("video", "pending"))
}

test_doctor_cannot_schedule_other_doctors_video if {
	not policies.allow with input as request("schedule_consultation", doctor, other_doctors("video", "pending"))
}

test_doctor_cannot_schedule_twice if {
	not policies.allow with input as request("schedule_consultation", doctor, own("video", "scheduled"))
}

test_doctor_cannot_schedule_written if {
	not policies.allow with input as request("schedule_consultation", doctor, own("written", "pending"))
}

test_admin_cannot_schedule if {
	not policies.allow with input as request("schedule_consultation", admin, own("video", "pending"))
}

# --- US-08: joining the call and the opinion after it ---

test_doctor_joins_own_call_in_window if {
	policies.allow with input as request("join_video_consultation", doctor, own_in_join_window("video", "scheduled"))
}

test_doctor_cannot_join_outside_window if {
	not policies.allow with input as request("join_video_consultation", doctor, own("video", "scheduled"))
}

test_doctor_cannot_join_other_doctors_call if {
	target := object.union(own_in_join_window("video", "scheduled"), {"UserID": "other-doctor-id"})
	not policies.allow with input as request("join_video_consultation", doctor, target)
}

test_admin_cannot_join_call if {
	not policies.allow with input as request("join_video_consultation", admin, own_in_join_window("video", "scheduled"))
}

test_doctor_gives_opinion_after_call if {
	policies.allow with input as request("answer_consultation", doctor, own_after_appointment("video", "scheduled"))
}

test_doctor_cannot_give_opinion_before_call if {
	not policies.allow with input as request("answer_consultation", doctor, own("video", "scheduled"))
}

# --- US-07: the patient's token is checked by the handler, these rules only cover the state ---

test_patient_joins_in_window if {
	policies.allow with input as request("join_video_consultation_patient", anonymous, own_in_join_window("video", "scheduled"))
}

test_patient_cannot_join_outside_window if {
	not policies.allow with input as request("join_video_consultation_patient", anonymous, own("video", "scheduled"))
}

test_patient_cannot_join_finished_call if {
	not policies.allow with input as request("join_video_consultation_patient", anonymous, own_in_join_window("video", "finished"))
}

# --- US-11: missed consultations are for the administrator ---

test_admin_sees_missed_consultations if {
	policies.allow with input as request("view_not_attended", admin, no_target)
}

test_doctor_cannot_list_missed_consultations if {
	not policies.allow with input as request("view_not_attended", doctor, no_target)
}

test_doctor_cannot_open_own_missed_consultation if {
	not policies.allow with input as request("consultation", doctor, own("written", "not_attended"))
}

test_doctor_cannot_answer_own_missed_consultation if {
	not policies.allow with input as request("answer_consultation", doctor, own("written", "not_attended"))
}

test_admin_opens_missed_consultation if {
	policies.allow with input as request("consultation", admin, own("written", "not_attended"))
}

# --- adding consultations by hand, for the testing environment ---

test_admin_adds_consultation if {
	policies.allow with input as request("add_consultation", admin, no_target)
}

test_doctor_cannot_add_consultation if {
	not policies.allow with input as request("add_consultation", doctor, no_target)
}
