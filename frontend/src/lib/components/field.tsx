import type { ReactNode } from "react";

/** The attributes that tie a control to its label, its hint, and its messages. */
export interface ControlProps {
  id: string;
  name: string;
  "aria-describedby"?: string;
  "aria-invalid"?: true;
}

/**
 * One field of a form: its label, a hint, the message of what is wrong with
 * it, and the control that `children` renders with the attributes that tie
 * them together. A field that a notice of the whole form is about points at
 * the notice too, so that a screen reader reads it with the field.
 */
export function Field({
  form,
  name,
  label,
  hint,
  error,
  noticeId,
  children,
}: {
  /** A name of the form that is unique on the page; it prefixes the ids. */
  form: string;
  name: string;
  label: string;
  hint?: ReactNode;
  error?: string | undefined;
  /** The id of the notice of the form, when the notice is about this field. */
  noticeId?: string | undefined;
  children: (control: ControlProps) => ReactNode;
}) {
  const id = `${form}-${name}`;
  const hintId = hint === undefined ? undefined : `${id}-hint`;
  const errorId = error === undefined ? undefined : `${id}-error`;
  const describedBy = [errorId, noticeId, hintId].filter((part) => part !== undefined).join(" ");
  const control: ControlProps = { id, name };
  if (describedBy !== "") {
    control["aria-describedby"] = describedBy;
  }
  if (error !== undefined || noticeId !== undefined) {
    control["aria-invalid"] = true;
  }
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {hint === undefined ? null : (
        <p id={hintId} className="hint">
          {hint}
        </p>
      )}
      {error === undefined ? null : (
        <p id={errorId} className="field-error">
          {error}
        </p>
      )}
      {children(control)}
    </div>
  );
}
