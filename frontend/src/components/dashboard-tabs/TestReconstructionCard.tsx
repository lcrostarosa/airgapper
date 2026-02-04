import { useState } from "react";
import { fromHex, combine, toHex } from "../../lib/sss";
import { Alert } from "../ui";
import type { VaultConfig } from "../../types";

interface TestReconstructionCardProps {
  config: VaultConfig;
}

export function TestReconstructionCard({ config }: TestReconstructionCardProps) {
  const [testShare, setTestShare] = useState("");
  const [testResult, setTestResult] = useState<string | null>(null);

  const testReconstruct = () => {
    if (!testShare.trim() || !config.localShare) {
      setTestResult("Enter both shares to test");
      return;
    }

    try {
      const share1 = {
        index: config.shareIndex!,
        data: fromHex(config.localShare),
      };
      const share2 = {
        index: config.shareIndex === 1 ? 2 : 1,
        data: fromHex(testShare.trim()),
      };

      const reconstructed = combine([share1, share2]);
      const reconstructedHex = toHex(reconstructed);

      if (config.password && reconstructedHex === config.password) {
        setTestResult("Success! Password reconstructed correctly.");
      } else if (config.password) {
        setTestResult("Mismatch! The reconstructed password doesn't match.");
      } else {
        setTestResult(`Reconstructed: ${reconstructedHex}`);
      }
    } catch (error) {
      setTestResult("Error: " + (error as Error).message);
    }
  };

  return (
    <div className="bg-gray-800 rounded-lg p-6 mb-6">
      <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
        <span>🧪</span> Test Key Reconstruction
      </h2>
      <p className="text-sm text-gray-400 mb-4">
        Verify that the shares can reconstruct the password correctly.
      </p>

      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium mb-2">
            Peer's Share (Index {config.shareIndex === 1 ? 2 : 1})
          </label>
          <input
            type="text"
            value={testShare}
            onChange={(e) => setTestShare(e.target.value)}
            placeholder="Paste the peer's share to test reconstruction"
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors font-mono text-sm"
          />
        </div>

        <button
          onClick={testReconstruct}
          disabled={!testShare.trim()}
          className="bg-purple-600 hover:bg-purple-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-2 px-4 rounded-lg transition-colors"
        >
          Test Reconstruction
        </button>

        {testResult && (
          <Alert
            variant={
              testResult.startsWith("Success")
                ? "success"
                : testResult.startsWith("Error") || testResult.startsWith("Mismatch")
                ? "error"
                : "info"
            }
          >
            {testResult}
          </Alert>
        )}
      </div>
    </div>
  );
}
