"use client";

// A failure that no screen expects, such as a bug. An API that fails is not
// one: every screen shows the problem of the API where it happens.
export default function ErrorPage({ retry }: { error: unknown; retry: () => void }) {
  return (
    <>
      <h1>Something went wrong</h1>
      <p>The page could not be shown. Try again, and if it fails again, come back later.</p>
      <button
        type="button"
        onClick={() => {
          retry();
        }}
      >
        Try again
      </button>
    </>
  );
}
