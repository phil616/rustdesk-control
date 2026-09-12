import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { Select } from "antd";
import chinese from "./zh-CN.json";
export type Language = "en-US" | "zh-CN";
export type Translate = (
  key: string,
  values?: Record<string, string | number>,
) => string;
const storageKey = "rdc.language";
const english: Record<string, string> = {
  approve: "Approve",
  reject: "Reject",
  "rotate-password": "Rotate Password",
  "reset-identity": "Reset Identity",
  delete: "Delete",
  device_uuid: "Device UUID",
  rustdesk_id: "RustDesk ID",
  hostname: "Hostname",
  os: "OS",
  os_version: "OS version",
  arch: "Architecture",
  rustdesk_version: "RustDesk version",
  managed_client_version: "Managed client version",
  status: "Approval status",
  first_seen_at: "First seen",
  last_seen_at: "Last seen",
  applied_policy_version: "Applied policy version",
  applied_password_version: "Applied password version",
  password_version: "Password version",
  online: "Online",
  password_synced: "Password synced",
};
function initialLanguage(): Language {
  try {
    const saved = localStorage.getItem(storageKey);
    if (saved === "zh-CN" || saved === "en-US") return saved;
  } catch {
    /* Storage may be disabled. */
  }
  return navigator.language.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US";
}
const Context = createContext<{
  language: Language;
  setLanguage: (value: Language) => void;
  t: Translate;
  date: (value: number) => string;
  errorText: (value: unknown) => string;
} | null>(null);
export function I18nProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState<Language>(initialLanguage);
  const t: Translate = (key, values) => {
    const dictionary: Record<string, string> =
      language === "zh-CN" ? chinese : english;
    return (dictionary[key] ?? key).replace(/\{(\w+)\}/g, (match, name) =>
      String(values?.[name] ?? match),
    );
  };
  const setLanguage = (value: Language) => {
    setLanguageState(value);
    try {
      localStorage.setItem(storageKey, value);
    } catch {
      /* Switching still works without storage. */
    }
  };
  useEffect(() => {
    document.documentElement.lang = language;
  }, [language]);
  const errorText = (value: unknown) =>
    t(
      value instanceof TypeError
        ? "Network request failed"
        : value instanceof Error
          ? value.message
          : String(value),
    );
  return (
    <Context.Provider
      value={{
        language,
        setLanguage,
        t,
        date: (value) => new Date(value * 1000).toLocaleString(language),
        errorText,
      }}
    >
      {children}
    </Context.Provider>
  );
}
export function useI18n() {
  const value = useContext(Context);
  if (!value) throw new Error("I18nProvider missing");
  return value;
}
export function LanguageSwitcher() {
  const { language, setLanguage } = useI18n();
  return (
    <Select
      aria-label="Language / 语言"
      value={language}
      onChange={setLanguage}
      style={{ width: 112 }}
      options={[
        { value: "zh-CN", label: "简体中文" },
        { value: "en-US", label: "English" },
      ]}
    />
  );
}
