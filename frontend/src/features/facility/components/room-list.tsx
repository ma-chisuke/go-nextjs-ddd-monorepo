import { ProblemNotice } from "@/lib/components/problem-notice";

import { listRooms } from "../server/rooms";

/**
 * The meeting rooms. It reads them from the Facility API when the page is
 * requested, so the page that shows it wraps it in <Suspense>.
 */
export async function RoomList() {
  const rooms = await listRooms();
  if (!rooms.ok) {
    return <ProblemNotice problem={rooms.problem} />;
  }
  if (rooms.value.length === 0) {
    return <p>No room is registered yet.</p>;
  }
  return (
    <ul className="cards" aria-label="Rooms">
      {rooms.value.map((room) => (
        <li key={room.id} className="card">
          <h2>{room.name}</h2>
          <dl>
            <dt>Capacity</dt>
            <dd>{room.capacity} people</dd>
            <dt>Open</dt>
            <dd>
              {room.openFrom} to {room.openUntil} ({room.timeZone})
            </dd>
          </dl>
        </li>
      ))}
    </ul>
  );
}
