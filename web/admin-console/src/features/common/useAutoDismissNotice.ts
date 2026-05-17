import { useEffect, useState, type Dispatch, type SetStateAction } from "react";

export type NoticeState = {
  ok: boolean;
  message: string;
};

type AutoDismissNoticeOptions = {
  successMillis?: number;
  errorMillis?: number;
};

const defaultSuccessMillis = 3500;
const defaultErrorMillis = 6500;

// Keeps page feedback lightweight: confirmations disappear quickly, while
// errors stay long enough to be read without permanently occupying the editor.
export function useAutoDismissNotice<TNotice extends NoticeState>(
  options: AutoDismissNoticeOptions = {}
): readonly [TNotice | null, Dispatch<SetStateAction<TNotice | null>>] {
  const [notice, setNotice] = useState<TNotice | null>(null);
  const successMillis = options.successMillis ?? defaultSuccessMillis;
  const errorMillis = options.errorMillis ?? defaultErrorMillis;

  useEffect(() => {
    if (!notice) return;

    const timeout = window.setTimeout(
      () => setNotice(null),
      notice.ok ? successMillis : errorMillis
    );
    return () => window.clearTimeout(timeout);
  }, [notice, successMillis, errorMillis]);

  return [notice, setNotice] as const;
}
