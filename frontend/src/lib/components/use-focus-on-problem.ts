import { useEffect, type RefObject } from "react";

/**
 * After each submit of a form, moves the focus to where the person has to
 * act: the first field that is not valid, or else the notice of the form. A
 * screen reader then reads the field with its message. `state` is what the
 * Server Function returned; a new one means a new submit. Only for client
 * components.
 */
export function useFocusOnProblem(
  state: unknown,
  form: RefObject<HTMLFormElement | null>,
  notice: RefObject<HTMLElement | null>,
): void {
  useEffect(() => {
    const invalid = form.current?.querySelector<HTMLElement>('[aria-invalid="true"]');
    if (invalid) {
      invalid.focus();
    } else {
      notice.current?.focus();
    }
  }, [state, form, notice]);
}
