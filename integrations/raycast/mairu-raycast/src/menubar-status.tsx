import { MenuBarExtra, Icon, showToast, Toast } from "@raycast/api";
import { useState, useEffect } from "react";
import { runMairuCmd } from "./mairu-cli";

interface SysInfo {
  os?: string;
  arch?: string;
  num_cpu?: number;
  mem_mb?: number;
  disk_free_gb?: number;
  disk_total_gb?: number;
  go_version?: string;
  docker?: boolean;
}

export default function Command() {
  const [isLoading, setIsLoading] = useState(true);
  const [sysInfo, setSysInfo] = useState<SysInfo | null>(null);

  useEffect(() => {
    async function fetchStatus() {
      try {
        const stdout = await runMairuCmd(`sys`);
        setSysInfo(JSON.parse(stdout));
      } catch (error: Error | unknown) {
        await showToast({
          style: Toast.Style.Failure,
          title: "Error fetching mairu sys",
          message: (error as Error).message,
        });
      } finally {
        setIsLoading(false);
      }
    }

    fetchStatus();
  }, []);

  const title = isLoading
    ? "Loading..."
    : sysInfo
      ? `Mairu (${sysInfo.mem_mb}MB)`
      : "Mairu: Error";
  const icon = sysInfo && sysInfo.docker ? Icon.Checkmark : Icon.XMarkCircle;

  return (
    <MenuBarExtra icon={icon} title={title} isLoading={isLoading}>
      {sysInfo ? (
        <>
          <MenuBarExtra.Item title={`OS: ${sysInfo.os} (${sysInfo.arch})`} />
          <MenuBarExtra.Item title={`CPU: ${sysInfo.num_cpu} cores`} />
          <MenuBarExtra.Item title={`Memory: ${sysInfo.mem_mb} MB used`} />
          <MenuBarExtra.Item
            title={`Disk: ${sysInfo.disk_free_gb?.toFixed(1)} GB free / ${sysInfo.disk_total_gb?.toFixed(1)} GB total`}
          />
          <MenuBarExtra.Item
            title={`Docker: ${sysInfo.docker ? "Running" : "Stopped"}`}
          />
          <MenuBarExtra.Item title={`Go Version: ${sysInfo.go_version}`} />
        </>
      ) : (
        <MenuBarExtra.Item title="Failed to load Mairu status" />
      )}
    </MenuBarExtra>
  );
}
