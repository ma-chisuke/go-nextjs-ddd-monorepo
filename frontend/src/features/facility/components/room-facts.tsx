import type { Room } from "../server/rooms";

/** What a room is: how many people it holds, and when it is open, in which time zone. */
export function RoomFacts({ room }: { room: Room }) {
  return (
    <dl>
      <dt>Capacity</dt>
      <dd>{room.capacity === 1 ? "1 person" : `${String(room.capacity)} people`}</dd>
      <dt>Open</dt>
      <dd>
        {room.openFrom} to {room.openUntil}
      </dd>
      <dt>Time zone</dt>
      <dd>{room.timeZone}</dd>
    </dl>
  );
}
