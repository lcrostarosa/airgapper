import { useState, useEffect } from "react";
import type { VaultConfig, Step } from "./types";
import { Welcome } from "./components/Welcome";
import { InitVault } from "./components/InitVault";
import { JoinVault } from "./components/JoinVault";
import { Dashboard } from "./components/Dashboard";

const STORAGE_KEY = "airgapper_vault";

function App() {
  const [step, setStep] = useState<Step>("welcome");
  const [config, setConfig] = useState<VaultConfig | null>(null);

  // Load saved config on mount
  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      try {
        const parsed = JSON.parse(saved);
        setConfig(parsed);
      } catch {
        localStorage.removeItem(STORAGE_KEY);
      }
    }
  }, []);

  const handleComplete = (newConfig: VaultConfig) => {
    setConfig(newConfig);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(newConfig));
    setStep("dashboard");
  };

  const handleClear = () => {
    if (confirm("Are you sure you want to reset the vault? Make sure you have backed up your key share!")) {
      setConfig(null);
      localStorage.removeItem(STORAGE_KEY);
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
          <JoinVault onComplete={handleComplete} onNavigate={setStep} />
        )}
        {step === "dashboard" && config && (
          <Dashboard config={config} onClear={handleClear} />
        )}
      </div>

      <footer className="fixed bottom-0 left-0 right-0 bg-gray-800/80 backdrop-blur py-2 px-4 text-center text-sm text-gray-500">
        Airgapper v0.3.0 • Keys stored in browser localStorage •{" "}
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
