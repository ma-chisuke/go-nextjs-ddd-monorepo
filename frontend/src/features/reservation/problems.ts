// What the forms of Reservation say about each kind of problem of the
// Reservation API. The kinds come from docs/problems/ and the README of the
// context: a kind that a person can act on gets words of its own, at the
// field that it is about where there is one. Any other problem shows its
// title and detail as they are. Nothing here reads a title or a detail.

import {
  problemFieldErrors,
  problemNotice,
  type FieldErrors,
  type Notice,
} from "../../lib/input.ts";
import { problemKind, type Problem } from "../../lib/problem.ts";

/** The fields of the forms of Reservation. */
export type ReservationField = "date" | "start" | "end" | "attendees" | "organizerEmail";

/** What a form asked the API to do. */
export type Operation = "book" | "reschedule" | "cancel";

/** What a form shows about a problem. */
export interface ProblemWords {
  fieldErrors: FieldErrors<ReservationField>;
  /** A message about the whole form. */
  notice?: Notice;
  /** The fields that the notice is about. */
  noticeFields?: ReservationField[];
  /**
   * Whether the reservation is not what the page showed: somebody changed or
   * cancelled it meanwhile. The page then shows it again as it is now.
   */
  stale: boolean;
}

// The field of the forms for each part of a request, as the pointers of a
// problem name them.
const fieldOfPart: Partial<Record<string, ReservationField>> = {
  startsAt: "start",
  endsAt: "end",
  attendees: "attendees",
  organizerEmail: "organizerEmail",
};

const period: ReservationField[] = ["start", "end"];

/** What a form of Reservation says about a problem of the Reservation API. */
export function reservationProblem(problem: Problem, operation: Operation): ProblemWords {
  const words = (notice: Notice, noticeFields: ReservationField[] = []): ProblemWords => ({
    fieldErrors: {},
    notice,
    noticeFields,
    stale: false,
  });
  const atField = (field: ReservationField, message: string): ProblemWords => ({
    fieldErrors: { [field]: message },
    stale: false,
  });
  switch (problemKind(problem)) {
    case "room-already-booked":
      return words(
        {
          title: "The room is already booked for part of this period.",
          detail: "Choose a period that the reservations of the day leave free.",
        },
        period,
      );
    case "outside-opening-hours":
      return words(
        {
          title: "The room is not open for the whole period.",
          detail: "Choose a period inside the opening hours of the room.",
        },
        period,
      );
    case "period-misaligned":
      return words(
        {
          title: "The period must start and end on a quarter hour.",
          detail: "Use :00, :15, :30, or :45 past the hour, such as 10:00 to 11:15.",
        },
        period,
      );
    case "period-too-long":
      return words(
        { title: "A reservation lasts at most 4 hours.", detail: "Choose a shorter period." },
        period,
      );
    case "period-out-of-order":
      return atField("end", "The end must be later than the start.");
    case "period-in-past":
      return operation === "reschedule"
        ? words(
            {
              title: "The period starts in the past, or the reservation has already started.",
              detail:
                "Choose a period that starts later than now. A reservation that has started cannot be rescheduled.",
            },
            ["start"],
          )
        : words(
            {
              title: "The period starts in the past.",
              detail: "Choose a period that starts later than now.",
            },
            ["start"],
          );
    case "over-capacity":
      return atField("attendees", "More people attend than the room holds.");
    case "no-attendees":
      return atField("attendees", "At least 1 person attends, the organizer included.");
    case "facility-unavailable":
      return words({
        title: "The room cannot be checked right now.",
        detail:
          operation === "book"
            ? "Nothing was booked. Wait a moment, then send the form again: sending it again never books the room twice."
            : "Nothing was changed. Wait a moment, then send the form again.",
      });
    case "cancellation-too-late":
      return words({
        title: "It is too late to cancel this reservation.",
        detail: "A reservation can be cancelled until 30 minutes before it starts.",
      });
    case "precondition-failed":
      return {
        ...words({
          title: "Somebody changed this reservation after you opened this page.",
          detail:
            "Nothing was changed. The page now shows the reservation as it is; make your change again if you still want it.",
        }),
        stale: true,
      };
    case "reservation-cancelled":
      return {
        ...words({
          title: "This reservation is cancelled.",
          detail: "A cancelled reservation cannot be changed. Book the room again to use it.",
        }),
        stale: true,
      };
  }
  const fieldErrors = problemFieldErrors(problem, fieldOfPart);
  if (Object.keys(fieldErrors).length > 0) {
    return { fieldErrors, stale: false };
  }
  return words(problemNotice(problem));
}
