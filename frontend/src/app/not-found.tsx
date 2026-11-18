import Link from "next/link";

export default function NotFound() {
  return (
    <>
      <h1>Page not found</h1>
      <p>Nothing is at this address.</p>
      <p>
        <Link href="/">See all rooms</Link>
      </p>
    </>
  );
}
