/**
 * Inline SVG icons for operating systems and distributions.
 * 
 * Platform detection parses the agent's `os` and `platform` fields to select
 * the most specific icon available. Falls back to a generic terminal icon.
 */

interface IconProps {
    size?: number;
    color?: string;
}

// Windows

function WindowsModernIcon({ size = 16, color = "#0078D4"}: IconProps) {
    return (
        <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
            <path d="M1 3.2L6.5 2.5V7.5H1V3.2Z" fill={color} />
            <path d="M7.5 2.35L15 1V7.5H7.5V2.35Z" fill={color} />
            <path d="M1 8.5H6.5V13.5L1 12.8V8.5Z" fill={color} />
            <path d="M7.5 8.5H15V15L7.5 13.65V8.5Z" fill={color} />
        </svg>
    );
}

function WindowsXPIcon({ size = 16 }: IconProps) {
    return (
        <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
            <path d="M1 3.2L6.5 2.5V7.5H1V3.2Z" fill="#FF0000" />
            <path d="M7.5 2.35L15 1V7.5H7.5V2.35Z" fill="#00B400" />
            <path d="M1 8.5H6.5V13.5L1 12.8V8.5Z" fill="#0058CF" />
            <path d="M7.5 8.5H15V15L7.5 13.65V8.5Z" fill="#FFB900" />
        </svg>
    );
}

// macOS

function MacOSIcon({ size = 16, color = "#a3a3a3" }: IconProps) {
    return (
        <svg width={size} height={size} viewBox="0 0 16 16" fill={color}>
            <path d="M11.2 8.5c0-1.7 1.4-2.5 1.5-2.6-.8-1.2-2.1-1.3-2.5-1.4-1.1-.1-2.1.6-2.6.6-.5 0-1.4-.6-2.2-.6C4 4.6 2.7 5.4 2 6.7
.6 9.3 1.6 13.1 3 15.2c.7 1 1.5 2.1 2.5 2.1 1 0 1.4-.7 2.6-.7s1.6.7 2.6.6c1.1 0 1.8-1 2.5-2 .8-1.1 1.1-2.2 1.1-2.3 0 0-2.1-.8-2.1-3.1zM9.3
3.4c.6-.7.9-1.6.8-2.6-.8 0-1.8.5-2.3 1.2-.5.6-.9 1.6-.8 2.5.9.1 1.8-.5 2.3-1.1z" transform="scale(0.65) translate(3.5, 1)" />
        </svg>
    );
}

// Linux

function TuxIcon({ size = 16, color = "#a3a3a3" }: IconProps) {
    return (
        <svg width={size} height={size} viewBox="0 0 16 16" fill={color}>
            <path d="M8 1C5.8 1 4.5 3.2 4.5 5.5c0 1.2.3 2 .8 2.8-.5.3-1.8 1.2-2.1 2.2-.3 1 .2 2.5 2.3 2.5.8 0 1.5-.2
2-.5.3.2.9.5 1.5.5s1.2-.3 1.5-.5c.5.3 1.2.5 2 .5 2.1 0 2.6-1.5 2.3-2.5-.3-1-1.6-1.9-2.1-2.2.5-.8.8-1.6.8-2.8C13.5 3.2
12.2 1 10 1H8zM6.5 4.5c.4 0 .8.4.8.8s-.4.8-.8.8-.8-.4-.8-.8.4-.8.8-.8zm3 0c.4 0
.8.4.8.8s-.4.8-.8.8-.8-.4-.8-.8.4-.8.8-.8zM7 7.5h2c0 .8-.4 1.2-1 1.2S7 8.3 7 7.5z" />
        </svg>
    );
}

function UbuntuIcon({ size = 16, color = "#e5e5e5" }: IconProps) {
    return (
        <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
            <ellipse cx="8" cy="9.5" rx="4" ry="4.5" fill="none" stroke={color} strokeWidth="1.3" />
            <circle cx="8" cy="5" r="2.8" fill="none" stroke={color} strokeWidth="1.3" />
            <circle cx="6.8" cy="4.5" r="0.6" fill={color} />
            <circle cx="9.2" cy="4.5" r="0.6" fill={color} />
            <path d="M7.2 5.8L8 6.3l.8-.5" stroke="#F5A623" strokeWidth="0.8" strokeLinecap="round" fill="none" />
        </svg>
    );
}

function DebianIcon({ size = 16 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="8" r="5.5" fill="none" stroke="#D70751" strokeWidth="1.5" />
      <path
        d="M8.5 4C10.5 4 11.5 5.5 11.5 7.5c0 2-1 3.5-3 3.5"
        stroke="#D70751"
        strokeWidth="1.5"
        strokeLinecap="round"
        fill="none"
      />
    </svg>
  );
}

function FedoraIcon({ size = 16 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="8" r="6" fill="none" stroke="#51A2DA" strokeWidth="1.5" />
      <path d="M8 4v4h4" stroke="#294172" strokeWidth="1.5" strokeLinecap="round" fill="none" />
      <circle cx="8" cy="8" r="1.2" fill="#294172" />
    </svg>
  );
}

function ArchIcon({ size = 16 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
      <path
        d="M8 1.5L3 14h2.2c.8-2 1.6-3.5 2.8-5 1.2 1.5 2 3 2.8 5H13L8 1.5z"
        fill="#1793D1"
      />
    </svg>
  );
}

function RHELIcon({ size = 16 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="8" r="6" fill="none" stroke="#EE0000" strokeWidth="1.5" />
      <path d="M5.5 5.5h3c.8 0 1.5.7 1.5 1.5s-.7 1.5-1.5 1.5H6.5v2" stroke="#EE0000" strokeWidth="1.3" strokeLinecap="round" fill="none" />
    </svg>
  );
}

function SUSEIcon({ size = 16 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="8" r="6" fill="none" stroke="#73BA25" strokeWidth="1.5" />
      <circle cx="6.5" cy="7" r="0.8" fill="#73BA25" />
      <circle cx="9.5" cy="7" r="0.8" fill="#73BA25" />
      <path d="M6 9.5c.5.8 1.2 1 2 1s1.5-.2 2-1" stroke="#73BA25" strokeWidth="0.8" strokeLinecap="round" fill="none" />
    </svg>
  );
}

function AlpineIcon({ size = 16 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
      <path d="M8 2L2 12h12L8 2z" fill="none" stroke="#0D597F" strokeWidth="1.3" />
      <path d="M8 5.5L5.5 10h5L8 5.5z" fill="#0D597F" />
    </svg>
  );
}

// FreeBSD

function FreeBSDIcon({ size = 16, color = "#AB2B28" }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
      <circle cx="8" cy="9" r="5.5" fill="none" stroke={color} strokeWidth="1.3" />
      <path d="M4.5 5c-.8-.8-1.5-2.5-.5-3.5s2.7-.3 3.5.5" stroke={color} strokeWidth="1" fill="none" />
      <path d="M11.5 5c.8-.8 1.5-2.5.5-3.5s-2.7-.3-3.5.5" stroke={color} strokeWidth="1" fill="none" />
    </svg>
  );
}

// Fallback

function GenericIcon({ size = 16, color = "#737373" }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
      <rect x="2" y="2" width="12" height="9" rx="1" stroke={color} strokeWidth="1.2" />
      <path d="M4 13h8" stroke={color} strokeWidth="1.2" strokeLinecap="round" />
      <path d="M4.5 5l1.5 1.5L4.5 8" stroke={color} strokeWidth="1" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M7.5 8H10" stroke={color} strokeWidth="1" strokeLinecap="round" />
    </svg>
  );
}

// Platform Detection

/**
 * Determine the appropriate OS icon based on agent os and platform fields.
 * 
 * Detection priority:
 *  1. Windows version (XP/Vista/7 - classic, 8+ - modern)
 *  2. macOS / darwin
 *  3. Linux distro by platform
 *  4. FreeBSD
 *  5. Generic fallback
 * 
 * @param os        Agent OS string ("Windows 11 Pro", "linux", etc.)
 * @param platform  Agent platform string ("debian", "ubuntu", "darwin")
 * @param size      Icon size in pixels.
 */
export function OSIcon({
    os,
    platform,
    size = 16,
}: {
    os: string;
    platform: string;
    size?: number;
}) {
    const osLower = os.toLowerCase();
    const platformLower = platform.toLowerCase();

    // Windows
    if (osLower.includes("windows")) {
        const xpEra = /windows (xp|vista|7|2000|2003|server 2003)/i;
        if (xpEra.test(os)) {
            return <WindowsXPIcon size={size} />;
        }
        return <WindowsModernIcon size={size} />;
    }

    // macOS
    if (osLower.includes("darwin") || osLower.includes("macos") || platformLower === "darwin") {
        return <MacOSIcon size={size} />;
    }

    // FreeBSD
    if (osLower.includes("freebsd") || platformLower.includes("freebsd")) {
        return <FreeBSDIcon size={size} />;
    }

    // Linux
    if (osLower === "linux" || platformLower === "linux" || platformLower !== "") {
        const p = platformLower;
        if (p.includes("ubuntu")) return <UbuntuIcon size={size} />;
        if (p.includes("debian") || p.includes("proxmox")) return <DebianIcon size={size} />;
        if (p.includes("fedora")) return <FedoraIcon size={size} />;
        if (p.includes("arch") || p.includes("manjaro")) return <ArchIcon size={size} />;
        if (p.includes("rhel") || p.includes("centos") || p.includes("rocky") || p.includes("alma") || p.includes("red hat")) return <RHELIcon size={size} />;
        if (p.includes("suse") || p.includes("sles")) return <SUSEIcon size={size} />;
        if (p.includes("alpine")) return <AlpineIcon size={size} />;
        if (p.includes("raspbian") || p.includes("raspberry")) return <DebianIcon size={size} />;
        if (osLower === "linux") return <TuxIcon size={size} />;
    }

    return <GenericIcon size={size} />;
}