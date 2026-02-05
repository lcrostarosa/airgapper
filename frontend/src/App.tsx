import { useState, useEffect } from "react";
import type { VaultConfig, Step } from "./types";
import { Welcome } from "./components/Welcome";
import { InitVault } from "./components/InitVault";
import { HostSetup } from "./components/HostSetup";
import { Dashboard } from "./components/Dashboard";

const STORAGE_KEY = "airgapper_vault";
const SESSION_TIMESTAMP_KEY = "airgapper_session_timestamp";
const SESSION_TIMEOUT_MS = 30 * 60 * 1000; // 30 minutes

// One-time migration from localStorage to sessionStorage
function migrateFromLocalStorage(): void {
  try {
    const localData = localStorage.getItem(STORAGE_KEY);
    if (localData && !sessionStorage.getItem(STORAGE_KEY)) {
      sessionStorage.setItem(STORAGE_KEY, localData);
      sessionStorage.setItem(SESSION_TIMESTAMP_KEY, Date.now().toString());
    }
    // Clear localStorage after migration (security improvement)
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    // Ignore migration errors
  }
}

function isSessionExpired(): boolean {
  const timestamp = sessionStorage.getItem(SESSION_TIMESTAMP_KEY);
  if (!timestamp) return false;
  return Date.now() - parseInt(timestamp, 10) > SESSION_TIMEOUT_MS;
}

function updateSessionTimestamp(): void {
  sessionStorage.setItem(SESSION_TIMESTAMP_KEY, Date.now().toString());
}

function loadSavedConfig(): VaultConfig | null {
  // Migrate from localStorage on first load
  migrateFromLocalStorage();

  // Check session expiry
  if (isSessionExpired()) {
    sessionStorage.removeItem(STORAGE_KEY);
    sessionStorage.removeItem(SESSION_TIMESTAMP_KEY);
    return null;
  }

  const saved = sessionStorage.getItem(STORAGE_KEY);
  if (saved) {
    try {
      updateSessionTimestamp();
      return JSON.parse(saved);
    } catch {
      sessionStorage.removeItem(STORAGE_KEY);
    }
  }
  return null;
}

function App() {
  const [step, setStep] = useState<Step>("welcome");
  const [config, setConfig] = useState<VaultConfig | null>(loadSavedConfig);

  // Set up activity tracking for session timeout
  useEffect(() => {
    const updateActivity = () => {
      if (config) {
        updateSessionTimestamp();
      }
    };

    // Update timestamp on user activity
    window.addEventListener("click", updateActivity);
    window.addEventListener("keypress", updateActivity);

    // Check for session expiry periodically
    const checkInterval = setInterval(() => {
      if (isSessionExpired()) {
        setConfig(null);
        sessionStorage.removeItem(STORAGE_KEY);
        sessionStorage.removeItem(SESSION_TIMESTAMP_KEY);
        setStep("welcome");
      }
    }, 60000); // Check every minute

    return () => {
      window.removeEventListener("click", updateActivity);
      window.removeEventListener("keypress", updateActivity);
      clearInterval(checkInterval);
    };
  }, [config]);

  const handleComplete = (newConfig: VaultConfig) => {
    setConfig(newConfig);
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(newConfig));
    updateSessionTimestamp();
    setStep("dashboard");
  };

  const handleClear = () => {
    if (confirm("Are you sure you want to reset the vault? Make sure you have backed up your key share!")) {
      setConfig(null);
      sessionStorage.removeItem(STORAGE_KEY);
      sessionStorage.removeItem(SESSION_TIMESTAMP_KEY);
      setStep("welcome");
    }
  };

  return (
    <div className="min-h-screen bg-gray-900 text-gray-100">
      <div className="container mx-auto px-4 py-8">
        {step === "welcome" && (
          <Welcome onNavigate={setStep} hasVault={config !== null} />
        )}
        {step === "init" && (
          <InitVault onComplete={handleComplete} onNavigate={setStep} />
        )}
        {step === "join" && (
          <HostSetup onComplete={handleComplete} onNavigate={setStep} />
        )}
        {step === "dashboard" && config && (
          <Dashboard config={config} onClear={handleClear} />
        )}
      </div>

      <footer className="fixed bottom-0 left-0 right-0 bg-gray-800/80 backdrop-blur py-2 px-4 text-center text-sm text-gray-500">
        Airgapper v0.3.0 • Keys stored in browser session (30 min timeout) •{" "}
        <a
          href="https://github.com/lcrostarosa/airgapper"
          target="_blank"
          rel="noopener noreferrer"
          className="text-blue-400 hover:text-blue-300"
        >
          GitHub
        </a>
      </footer>
    </div>
  );
}

export default App;
