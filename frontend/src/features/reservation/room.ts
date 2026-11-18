/**
 * A meeting room as far as a reservation needs to know it: what to call it,
 * how many people it holds, when it is open, and the time zone in which its
 * opening hours and the periods of its reservations are read (the word "room"
 * in the glossary of Reservation). Facility owns rooms; a page of the app
 * reads one through the facility feature and hands it to this feature in
 * these words, so that the two features never import each other (TPL-0003).
 */
export interface Room {
  id: string;
  name: string;
  capacity: number;
  /** The time of day from which the room can be used, such as "09:00". */
  openFrom: string;
  /** The time of day until which the room can be used, such as "18:00". */
  openUntil: string;
  /** The IANA time zone of the room, such as "Asia/Tokyo". */
  timeZone: string;
}
