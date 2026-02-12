import { useCallback, useEffect, useRef, useState } from "react";
import {
  ThemeProvider as HarmonyThemeProvider,
  Text,
  Paper,
  Flex,
  Avatar,
  Button,
  TextLink,
  IconValidationCheck,
  IconPlus,
  IconTrash,
  IconButton,
  IconKebabHorizontal,
  Tag,
  Tooltip,
  IconClose,
} from "@audius/harmony";
import { css } from "@emotion/react";
import { useSdk } from "./hooks/useSdk";
import { CreateKeyModal } from "./components/CreateKeyModal";
import { DeleteAppModal } from "./components/DeleteAppModal";

type OAuthUser = { userId: string; handle: string };

type ApiAccessKey = {
  api_access_key: string;
  is_active: boolean;
};

type DeveloperApp = {
  address: string;
  user_id: string;
  name: string;
  description: string | null;
  image_url: string | null;
  request_count?: number;
  request_count_all_time?: number;
  is_legacy?: boolean;
  api_access_keys?: ApiAccessKey[];
};

// API returns 150x150, 480x480; SDK uses _150x150, _480x480
type ProfilePictureData = {
  "150x150"?: string;
  "480x480"?: string;
  "1000x1000"?: string;
  _150x150?: string;
  _480x480?: string;
  _1000x1000?: string;
  mirrors?: string[];
};

type FullUser = {
  userId: string;
  handle: string;
  name?: string;
  profilePicture?: ProfilePictureData;
};

const API_BASE = import.meta.env.VITE_API_BASE ?? "https://api.audius.co";

const OAUTH_USER_KEY = "audius-api-plans-oauth-user";
const OAUTH_TOKEN_KEY = "@audius/sdk/token";

const messages = {
  navAudius: "audius.co",
  navApiDocs: "API Docs",
  navGithub: "GitHub",
  navDiscord: "Discord",
  audiusUrl: "https://audius.co",
  githubUrl: "https://github.com/audiusproject",
  discordUrl: "https://discord.gg/audius",
  title: "The Audius API",
  apiKeysSection: "Your API Keys",
  createKey: "Create Your First API Key",
  requestsThisMonth: "Requests this month",
  requestsAllTime: "Requests all time",
  noDeveloperApps: "No developer apps yet.",
  quickstart: "Quickstart",
  rest: "REST:",
  restEndpoint: "api.audius.co",
  grpc: "GRPC:",
  grpcEndpoint: "grpc.audius.co:443",
  apiDocs: "API Docs:",
  apiDocsUrl: "docs.audius.co/api",
  swaggerDefinition: "Swagger Definition:",
  swaggerUrl: "api.audius.co/v1",
  freePlan: "Free",
  unlimitedPlan: "Unlimited",
  freeSubtitle: "No Restrictions. Always Free.",
  unlimitedSubtitle: "Need Support and Higher Limits?",
  readTheDocs: "Read The Docs",
  createApiKey: "Create API Key",
  login: "Login",
  logout: "Log Out",
  contactUs: "Contact Us",
  apiKeyLabel: "API Key",
  apiSecretLabel: "API Bearer Token",
  copy: "Copy",
  copied: "Copied!",
  usageDetails: "USAGE DETAILS",
  online: "ONLINE",
  apiDocsFullUrl: "https://docs.audius.co/api",
  contactEmail: "api@audius.co",
  rateLimits: "Rate Limits:",
  requestsPerSecond: "Requests/Second",
  requestsPerMonth: "Requests/Month",
  freeRateLimit: "10",
  freeMonthlyLimit: "500,000",
  unlimitedRateLimit: "Unlimited",
  unlimitedMonthlyLimit: "Unlimited",
  legacyPill: "LEGACY app",
  legacyTooltip:
    "Sign API requests using your API Secret Key in the Authorization Header",
  createNewKey: "Create New Key",
  deleteApp: "Delete API Key",
  newBearerToken: "New Bearer Token",
  revokeToken: "Revoke Bearer Token",
};

const planGraphicSize = 64;

/** Black circle overlay with inverted content - used when hovering "Open Audio Protocol" */
const GrayscaleOverlay = ({
  x,
  y,
  isExiting,
  onComplete,
}: {
  x: number;
  y: number;
  isExiting: boolean;
  onComplete: () => void;
}) => {
  const [radius, setRadius] = useState(0);
  const radiusRef = useRef(0);

  useEffect(() => {
    const durationMs = 1800;
    const start = performance.now();
    const animate = (now: number) => {
      const elapsed = now - start;
      const t = Math.min(elapsed / durationMs, 1);
      const eased = 1 - (1 - t) * (1 - t);
      const r = eased * 150;
      radiusRef.current = r;
      setRadius(r);
      if (t < 1) requestAnimationFrame(animate);
    };
    requestAnimationFrame(animate);
  }, []);

  useEffect(() => {
    if (!isExiting) return;
    const durationMs = 1800;
    const startRadius = radiusRef.current;
    const start = performance.now();
    const animate = (now: number) => {
      const elapsed = now - start;
      const t = Math.min(elapsed / durationMs, 1);
      const eased = 1 - t * t;
      const r = startRadius * eased;
      radiusRef.current = r;
      setRadius(r);
      if (t < 1) {
        requestAnimationFrame(animate);
      } else {
        onComplete();
      }
    };
    requestAnimationFrame(animate);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only run when isExiting becomes true
  }, [isExiting]);
  return (
    <div
      aria-hidden
      css={css`
        position: fixed;
        inset: 0;
        pointer-events: none;
        z-index: 9999;
        /* Black circle with white text (invert + grayscale + push to pure white) */
        background: rgba(0, 0, 0, 0.35);
        -webkit-backdrop-filter: invert(1) saturate(0) brightness(2.5)
          contrast(1.2);
        backdrop-filter: invert(1) saturate(0) brightness(2.5) contrast(1.2);
        mask-image: radial-gradient(
          circle at ${x}px ${y}px,
          black 0,
          black ${radius}vmax,
          transparent ${radius}vmax
        );
        -webkit-mask-image: radial-gradient(
          circle at ${x}px ${y}px,
          black 0,
          black ${radius}vmax,
          transparent ${radius}vmax
        );
        mask-size: 100% 100%;
      `}
    />
  );
};

const FreePlanGraphic = () => (
  <svg
    width={planGraphicSize}
    height={planGraphicSize}
    viewBox="0 0 64 48"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    css={css`
      opacity: 0.9;
      flex-shrink: 0;
    `}
  >
    <defs>
      <linearGradient id="freePlanGrad" x1="0%" y1="100%" x2="0%" y2="0%">
        <stop offset="0%" stopColor="var(--harmony-primary-primary, #7d2fe0)" />
        <stop offset="100%" stopColor="#b366ff" />
      </linearGradient>
    </defs>
    <rect
      x="12"
      y="28"
      width="16"
      height="10"
      rx="2"
      fill="url(#freePlanGrad)"
      opacity="0.7"
    />
    <rect
      x="22"
      y="18"
      width="14"
      height="10"
      rx="2"
      fill="url(#freePlanGrad)"
      opacity="0.85"
    />
    <rect
      x="32"
      y="8"
      width="12"
      height="10"
      rx="2"
      fill="url(#freePlanGrad)"
    />
  </svg>
);

const UnlimitedPlanGraphic = () => (
  <svg
    width={planGraphicSize}
    height={planGraphicSize}
    viewBox="0 0 64 48"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    css={css`
      opacity: 0.9;
      flex-shrink: 0;
    `}
  >
    <defs>
      <linearGradient
        id="unlimitedPlanGrad"
        x1="0%"
        y1="50%"
        x2="100%"
        y2="50%"
      >
        <stop offset="0%" stopColor="var(--harmony-primary-primary, #7d2fe0)" />
        <stop offset="100%" stopColor="#b366ff" />
      </linearGradient>
    </defs>
    <circle
      cx="32"
      cy="24"
      r="6"
      stroke="url(#unlimitedPlanGrad)"
      strokeWidth="2"
      fill="none"
      opacity="0.9"
    />
    <circle
      cx="32"
      cy="24"
      r="14"
      stroke="url(#unlimitedPlanGrad)"
      strokeWidth="1.5"
      fill="none"
      opacity="0.6"
    />
    <circle
      cx="32"
      cy="24"
      r="22"
      stroke="url(#unlimitedPlanGrad)"
      strokeWidth="1"
      fill="none"
      opacity="0.35"
    />
  </svg>
);

function getPrimaryProfilePictureUrl(
  profilePicture?: ProfilePictureData | null,
): string | undefined {
  if (!profilePicture) return undefined;
  return (
    profilePicture["480x480"] ??
    profilePicture._480x480 ??
    profilePicture["150x150"] ??
    profilePicture._150x150 ??
    profilePicture["1000x1000"] ??
    profilePicture._1000x1000
  );
}

function getProfilePictureMirrors(
  profilePicture?: ProfilePictureData | null,
): string[] {
  return profilePicture?.mirrors ?? [];
}

function ProfileAvatar({
  profilePicture,
  size = "medium",
}: {
  profilePicture?: ProfilePictureData | null;
  size?: "small" | "medium" | "large";
}) {
  const primaryUrl = getPrimaryProfilePictureUrl(profilePicture);
  const mirrors = getProfilePictureMirrors(profilePicture);
  const [currentSrc, setCurrentSrc] = useState<string | undefined>(primaryUrl);
  const [mirrorIndex, setMirrorIndex] = useState(0);

  useEffect(() => {
    setCurrentSrc(primaryUrl);
    setMirrorIndex(0);
  }, [primaryUrl]);

  const handleError = () => {
    if (mirrorIndex < mirrors.length && primaryUrl) {
      const mirror = mirrors[mirrorIndex];
      try {
        const primaryPath = new URL(primaryUrl).pathname;
        const mirrorBase = mirror.endsWith("/") ? mirror.slice(0, -1) : mirror;
        setCurrentSrc(mirrorBase + primaryPath);
      } catch {
        setCurrentSrc(mirror);
      }
      setMirrorIndex((i) => i + 1);
    } else {
      setCurrentSrc(undefined);
    }
  };

  const sizePx = size === "small" ? 24 : size === "medium" ? 32 : 48;
  if (currentSrc == null) {
    return (
      <Flex
        w={sizePx}
        h={sizePx}
        css={css`
          border-radius: 50%;
          background: var(--harmony-primary-primary, #7d2fe0);
          flex-shrink: 0;
        `}
      />
    );
  }

  return <Avatar size={size} src={currentSrc} onError={handleError} />;
}

const PLACEHOLDER_COLORS = [
  "#FF6B6B",
  "#4ECDC4",
  "#45B7D1",
  "#96CEB4",
  "#FFEAA7",
  "#DDA0DD",
  "#98D8C8",
  "#F7DC6F",
  "#BB8FCE",
  "#85C1E9",
];

function hashString(str: string): number {
  let h = 0;
  for (let i = 0; i < str.length; i++) {
    h = (h << 5) - h + str.charCodeAt(i);
    h = h & h;
  }
  return Math.abs(h);
}

function CopyableCodeBlock({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <Flex
      direction="column"
      gap="xs"
      css={css`
        position: relative;
        width: 100%;
      `}
    >
      <Flex
        onClick={handleCopy}
        css={css`
          width: 100%;
          box-sizing: border-box;
          background: var(--harmony-neutral-neutral-1, #f8f8f8);
          padding: 1rem 1.25rem;
          padding-right: 5rem;
          border-radius: 8px;
          overflow-x: auto;
          font-size: 0.8125rem;
          font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace;
          line-height: 1.5;
          border: 1px solid var(--harmony-neutral-neutral-3, #e8e8e8);
          cursor: pointer;
          transition:
            background 0.15s,
            border-color 0.15s;
          &:hover {
            background: var(--harmony-neutral-neutral-2, #f0f0f0);
            border-color: var(--harmony-neutral-neutral-4, #d8d8d8);
          }
        `}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            handleCopy();
          }
        }}
        aria-label="Click to copy code"
      >
        <code>{code}</code>
      </Flex>
      <Flex
        css={css`
          position: absolute;
          top: 0.75rem;
          right: 0.75rem;
        `}
      >
        <Button
          variant="tertiary"
          size="xs"
          onClick={(e) => {
            e.stopPropagation();
            handleCopy();
          }}
          disabled={copied}
        >
          {copied ? messages.copied : messages.copy}
        </Button>
      </Flex>
    </Flex>
  );
}

function CopyableField({
  label,
  value,
  id,
  compact,
}: {
  label: string;
  value: string;
  id: string;
  compact?: boolean;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <Flex
      direction="column"
      gap="xs"
      css={
        compact
          ? css`
              flex: 1;
              min-width: 0;
            `
          : undefined
      }
    >
      <Text tag="label" variant="body" strength="strong" htmlFor={id}>
        {label}
      </Text>
      <Flex gap="s" alignItems="center">
        <input
          id={id}
          type="text"
          readOnly
          value={value}
          css={css`
            flex: 1;
            padding: 0.5rem 0.75rem;
            font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace;
            font-size: 0.875rem;
            border: 1px solid var(--harmony-neutral-neutral-3, #e0e0e0);
            border-radius: 4px;
            background: var(--harmony-background, #f5f5f5);
            min-width: 0;
          `}
        />
        <Button
          variant="secondary"
          size="small"
          onClick={handleCopy}
          disabled={copied}
        >
          {copied ? messages.copied : messages.copy}
        </Button>
      </Flex>
    </Flex>
  );
}

function BearerTokenField({
  value,
  id,
  onRevoke,
  onAdd,
}: {
  value: string;
  id: string;
  onRevoke: () => void | Promise<void>;
  onAdd: () => void | Promise<void>;
}) {
  const [copied, setCopied] = useState(false);
  const [pending, setPending] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  const handleRevoke = async () => {
    if (pending) return;
    setPending(true);
    try {
      await onRevoke();
    } catch (err) {
      console.error(err);
    } finally {
      setPending(false);
    }
  };

  const handleAdd = async () => {
    if (pending) return;
    setPending(true);
    try {
      await onAdd();
    } catch (err) {
      console.error(err);
    } finally {
      setPending(false);
    }
  };

  return (
    <Flex
      direction="column"
      gap="xs"
      css={css`
        flex: 1;
        min-width: 0;
      `}
    >
      <Text tag="label" variant="body" strength="strong" htmlFor={id}>
        {messages.apiSecretLabel}
      </Text>
      <Flex gap="s" alignItems="center">
        <input
          id={id}
          type="text"
          readOnly
          value={value}
          css={css`
            flex: 1;
            padding: 0.5rem 0.75rem;
            font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace;
            font-size: 0.875rem;
            border: 1px solid var(--harmony-neutral-neutral-3, #e0e0e0);
            border-radius: 4px;
            background: var(--harmony-background, #f5f5f5);
            min-width: 0;
          `}
        />
        <Button
          variant="secondary"
          size="small"
          onClick={handleCopy}
          disabled={copied}
        >
          {copied ? messages.copied : messages.copy}
        </Button>
        <span
          role="button"
          tabIndex={0}
          onClick={(e) => {
            e.stopPropagation();
            void handleRevoke();
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              void handleRevoke();
            }
          }}
          css={css`
            display: inline-flex;
            cursor: pointer;
          `}
        >
          <Tooltip text={messages.revokeToken} placement="top">
            <IconButton
              icon={IconClose}
              size="s"
              color="danger"
              aria-label={messages.revokeToken}
              disabled={pending}
            />
          </Tooltip>
        </span>
        <span
          role="button"
          tabIndex={0}
          onClick={(e) => {
            e.stopPropagation();
            void handleAdd();
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              void handleAdd();
            }
          }}
          css={css`
            display: inline-flex;
            cursor: pointer;
          `}
        >
          <Tooltip text={messages.newBearerToken} placement="top">
            <IconButton
              icon={IconPlus}
              size="s"
              color="success"
              aria-label={messages.newBearerToken}
              disabled={pending}
            />
          </Tooltip>
        </span>
      </Flex>
    </Flex>
  );
}

function AppAvatar({ app }: { app: DeveloperApp }) {
  const size = 24;
  if (app.image_url) {
    return <Avatar size="small" src={app.image_url} />;
  }
  const colorIndex =
    hashString(app.address || app.name || "?") % PLACEHOLDER_COLORS.length;
  const bgColor = PLACEHOLDER_COLORS[colorIndex];
  return (
    <Flex
      w={size}
      h={size}
      css={css`
        border-radius: 4px;
        background: ${bgColor};
        flex-shrink: 0;
      `}
    />
  );
}

export default function App() {
  const { sdk } = useSdk();

  const [oauthUser, setOauthUser] = useState<OAuthUser | null>(() => {
    try {
      const stored = sessionStorage.getItem(OAUTH_USER_KEY);
      if (stored) {
        const parsed = JSON.parse(stored) as OAuthUser;
        if (parsed?.userId && parsed?.handle) return parsed;
      }
    } catch {
      // ignore invalid stored data
    }
    return null;
  });
  const [fullUser, setFullUser] = useState<FullUser | null>(null);
  const [developerApps, setDeveloperApps] = useState<DeveloperApp[]>([]);
  const [createKeyModalOpen, setCreateKeyModalOpen] = useState(false);
  const [deleteAppModalApp, setDeleteAppModalApp] =
    useState<DeveloperApp | null>(null);
  const [navMenuOpen, setNavMenuOpen] = useState(false);
  const navMenuRef = useRef<HTMLSpanElement | null>(null);

  useEffect(() => {
    if (!navMenuOpen) return;
    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as Node;
      if (navMenuRef.current != null && !navMenuRef.current.contains(target)) {
        setNavMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [navMenuOpen]);

  // Grayscale "spotlight" effect when hovering Open Audio Protocol link
  const [grayscaleHover, setGrayscaleHover] = useState(false);
  const [grayscaleExiting, setGrayscaleExiting] = useState(false);
  const grayscaleOrigin = useRef({ x: 0, y: 0 });
  const openAudioLinkRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!grayscaleHover || grayscaleExiting) return;
    const checkLeave = (e: MouseEvent) => {
      const wrapper = openAudioLinkRef.current;
      if (!wrapper) return;
      const el = document.elementFromPoint(e.clientX, e.clientY);
      if (!el || !wrapper.contains(el)) {
        setGrayscaleExiting(true);
      }
    };
    document.addEventListener("mousemove", checkLeave, { passive: true });
    return () => document.removeEventListener("mousemove", checkLeave);
  }, [grayscaleHover, grayscaleExiting]);

  const handleLogin = () => {
    sdk.oauth?.login({ scope: "write" });
  };

  const handleLogout = () => {
    sessionStorage.removeItem(OAUTH_USER_KEY);
    sessionStorage.removeItem(OAUTH_TOKEN_KEY);
    setOauthUser(null);
    setFullUser(null);
    setDeveloperApps([]);
  };

  /**
   * Init @audius/sdk oauth
   */
  useEffect(() => {
    sdk.oauth?.init({
      successCallback: (
        profile: {
          userId?: string | number;
          sub?: string | number;
          handle?: string;
        },
        token?: string,
      ) => {
        const user: OAuthUser = {
          userId: String(profile.userId ?? profile.sub ?? ""),
          handle: profile.handle ?? "",
        };
        sessionStorage.setItem(OAUTH_USER_KEY, JSON.stringify(user));
        if (token) {
          sessionStorage.setItem(OAUTH_TOKEN_KEY, token);
        }
        setOauthUser(user);
      },
      errorCallback: (error: string) => console.log("Got error", error),
    });
  }, [sdk.oauth]);

  const loadDeveloperApps = useCallback(
    async (mergeApp?: DeveloperApp) => {
      if (!oauthUser?.userId) return;
      try {
        const appsRes = await fetch(
          `${API_BASE}/v1/users/${encodeURIComponent(oauthUser.userId)}/developer-apps?include=metrics`,
        );
        if (appsRes.ok) {
          const { data } = (await appsRes.json()) as { data: DeveloperApp[] };
          let apps = data ?? [];
          // If we have an optimistic app and the API doesn't include it yet (indexer lag), prepend it
          if (
            mergeApp?.address &&
            !apps.some(
              (a) =>
                a.address?.toLowerCase() === mergeApp.address?.toLowerCase(),
            )
          ) {
            apps = [mergeApp, ...apps];
          }
          setDeveloperApps(apps);
        }
      } catch {
        // ignore
      }
    },
    [oauthUser?.userId],
  );

  const handleCreateKey = useCallback(
    async (name: string) => {
      if (!oauthUser?.userId) throw new Error("Not logged in");
      const token = sessionStorage.getItem(OAUTH_TOKEN_KEY);
      const res = await fetch(
        `${API_BASE}/v1/users/${encodeURIComponent(oauthUser.userId)}/developer-apps`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({ name }),
        },
      );
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(
          (err as { error?: string })?.error ?? "Failed to create key",
        );
      }
      const result = (await res.json()) as {
        api_key?: string;
        api_secret?: string;
      };
      if (result.api_key != null && result.api_secret != null) {
        navigator.clipboard.writeText(
          `API Key: ${result.api_key}\nAPI Bearer Token: ${result.api_secret}`,
        );
      }
      // Optimistically add new app and reload; merge ensures it stays visible if indexer hasn't processed yet
      const optimisticApp: DeveloperApp = {
        address: result.api_key ?? "",
        user_id: oauthUser.userId,
        name,
        description: null,
        image_url: null,
        request_count: 0,
        request_count_all_time: 0,
        is_legacy: false,
        api_access_keys:
          result.api_secret != null
            ? [{ api_access_key: result.api_secret, is_active: true }]
            : [],
      };
      await loadDeveloperApps(optimisticApp);
      // Retry after delay to pick up real data once indexer processes the new app
      setTimeout(() => loadDeveloperApps(optimisticApp), 5000);
    },
    [oauthUser?.userId, loadDeveloperApps],
  );

  const handleDeactivateAccessKey = useCallback(
    async (app: DeveloperApp, apiAccessKey: string) => {
      if (oauthUser?.userId == null) throw new Error("Not logged in");
      const token = sessionStorage.getItem(OAUTH_TOKEN_KEY);
      const res = await fetch(
        `${API_BASE}/v1/users/${encodeURIComponent(oauthUser.userId)}/developer-apps/${encodeURIComponent(app.address)}/access-keys/deactivate`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(token != null ? { Authorization: `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({ api_access_key: apiAccessKey }),
        },
      );
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(
          (err as { error?: string })?.error ?? "Failed to revoke bearer token",
        );
      }
      setDeveloperApps((prev) =>
        prev.map((a) => {
          if (a.address?.toLowerCase() !== app.address?.toLowerCase()) return a;
          const keys =
            a.api_access_keys?.filter(
              (k) => k.api_access_key !== apiAccessKey,
            ) ?? [];
          return { ...a, api_access_keys: keys };
        }),
      );
    },
    [oauthUser?.userId],
  );

  const handleAddAccessKey = useCallback(
    async (app: DeveloperApp) => {
      if (oauthUser?.userId == null) throw new Error("Not logged in");
      const token = sessionStorage.getItem(OAUTH_TOKEN_KEY);
      const res = await fetch(
        `${API_BASE}/v1/users/${encodeURIComponent(oauthUser.userId)}/developer-apps/${encodeURIComponent(app.address)}/access-keys`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(token != null ? { Authorization: `Bearer ${token}` } : {}),
          },
        },
      );
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(
          (err as { error?: string })?.error ?? "Failed to create bearer token",
        );
      }
      const result = (await res.json()) as { api_access_key: string };
      if (result.api_access_key != null) {
        setDeveloperApps((prev) =>
          prev.map((a) => {
            if (a.address?.toLowerCase() !== app.address?.toLowerCase())
              return a;
            const keys = [...(a.api_access_keys ?? [])];
            keys.push({
              api_access_key: result.api_access_key,
              is_active: true,
            });
            return { ...a, api_access_keys: keys };
          }),
        );
        navigator.clipboard.writeText(result.api_access_key);
      }
    },
    [oauthUser?.userId],
  );

  const handleDeleteApp = useCallback(
    async (app: { address: string; name: string }) => {
      if (oauthUser?.userId == null) throw new Error("Not logged in");
      const token = sessionStorage.getItem(OAUTH_TOKEN_KEY);
      const res = await fetch(
        `${API_BASE}/v1/users/${encodeURIComponent(oauthUser.userId)}/developer-apps/${encodeURIComponent(app.address)}`,
        {
          method: "DELETE",
          headers: {
            ...(token != null ? { Authorization: `Bearer ${token}` } : {}),
          },
        },
      );
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(
          (err as { error?: string })?.error ?? "Failed to delete API key",
        );
      }
      setDeleteAppModalApp(null);
      setDeveloperApps((prev) => prev.filter((a) => a.address !== app.address));
      loadDeveloperApps();
    },
    [oauthUser?.userId, loadDeveloperApps],
  );

  /**
   * Fetch full user profile and developer apps when logged in.
   * Uses API (api.audius.co) for user - it returns proper CDN URLs for profile pictures.
   */
  useEffect(() => {
    if (!oauthUser?.userId) return;

    const load = async () => {
      try {
        const [userRes, appsRes] = await Promise.all([
          fetch(`${API_BASE}/v1/users/${encodeURIComponent(oauthUser.userId)}`),
          fetch(
            `${API_BASE}/v1/users/${encodeURIComponent(oauthUser.userId)}/developer-apps?include=metrics`,
          ),
        ]);
        const userData = userRes.ok
          ? (
              (await userRes.json()) as {
                data: {
                  id?: string;
                  handle?: string;
                  name?: string;
                  profile_picture?: ProfilePictureData;
                };
              }
            ).data
          : undefined;
        if (userData) {
          setFullUser({
            userId: userData.id ?? oauthUser.userId,
            handle: userData.handle ?? oauthUser.handle,
            name: userData.name,
            profilePicture: userData.profile_picture ?? undefined,
          });
        } else {
          setFullUser({
            userId: oauthUser.userId,
            handle: oauthUser.handle,
          });
        }
        if (appsRes.ok) {
          const { data } = (await appsRes.json()) as { data: DeveloperApp[] };
          setDeveloperApps(data ?? []);
        }
      } catch {
        setFullUser({
          userId: oauthUser.userId,
          handle: oauthUser.handle,
        });
      }
    };

    load();
  }, [oauthUser?.userId, oauthUser?.handle]);

  useEffect(() => {
    document.body.style.backgroundColor = "var(--harmony-background)";
    document.body.style.minHeight = "100vh";
  }, []);

  return (
    <HarmonyThemeProvider theme="day">
      <CreateKeyModal
        isOpen={createKeyModalOpen}
        onClose={() => setCreateKeyModalOpen(false)}
        onSubmit={handleCreateKey}
      />
      <DeleteAppModal
        isOpen={deleteAppModalApp != null}
        onClose={() => setDeleteAppModalApp(null)}
        app={deleteAppModalApp}
        onConfirm={handleDeleteApp}
      />
      {/* Grayscale spotlight overlay - B&W expands from cursor when hovering Open Audio Protocol */}
      {grayscaleHover ? (
        <GrayscaleOverlay
          x={grayscaleOrigin.current.x}
          y={grayscaleOrigin.current.y}
          isExiting={grayscaleExiting}
          onComplete={() => {
            setGrayscaleHover(false);
            setGrayscaleExiting(false);
          }}
        />
      ) : null}
      <Flex
        direction="column"
        gap="3xl"
        m="2xl"
        css={css`
          max-width: 1200px;
          margin: 0 auto;
          padding: 2rem;
        `}
      >
        {/* Nav */}
        <Flex
          gap="l"
          alignItems="center"
          justifyContent="space-between"
          css={css`
            padding-bottom: 1rem;
            border-bottom: 1px solid var(--harmony-neutral-neutral-3, #e8e8e8);
            position: relative;
          `}
        >
          <Flex
            gap="l"
            alignItems="center"
            css={css`
              @media (max-width: 600px) {
                display: none;
              }
            `}
          >
            <Text
              tag="a"
              href={messages.audiusUrl}
              target="_blank"
              rel="noopener noreferrer"
              variant="body"
              strength="strong"
              css={css`
                color: inherit;
                text-decoration: none;
                &:hover {
                  text-decoration: underline;
                }
              `}
            >
              {messages.navAudius}
            </Text>
            <Text
              tag="a"
              href={`https://${messages.apiDocsUrl}`}
              target="_blank"
              rel="noopener noreferrer"
              variant="body"
              strength="strong"
              css={css`
                color: inherit;
                text-decoration: none;
                &:hover {
                  text-decoration: underline;
                }
              `}
            >
              {messages.navApiDocs}
            </Text>
            <Text
              tag="a"
              href={messages.githubUrl}
              target="_blank"
              rel="noopener noreferrer"
              variant="body"
              strength="strong"
              css={css`
                color: inherit;
                text-decoration: none;
                &:hover {
                  text-decoration: underline;
                }
              `}
            >
              {messages.navGithub}
            </Text>
            <Text
              tag="a"
              href={messages.discordUrl}
              target="_blank"
              rel="noopener noreferrer"
              variant="body"
              strength="strong"
              css={css`
                color: inherit;
                text-decoration: none;
                &:hover {
                  text-decoration: underline;
                }
              `}
            >
              {messages.navDiscord}
            </Text>
          </Flex>
          {/* Mobile menu button + dropdown */}
          <Flex
            alignItems="center"
            css={css`
              display: none;
              @media (max-width: 600px) {
                display: flex;
              }
            `}
          >
            <span
              ref={navMenuRef}
              css={css`
                position: relative;
              `}
            >
              <IconButton
                icon={IconKebabHorizontal}
                aria-label="Menu"
                onClick={() => setNavMenuOpen((o) => !o)}
              />
              {navMenuOpen ? (
                <Paper
                  p="s"
                  css={css`
                    position: absolute;
                    top: 100%;
                    left: 0;
                    margin-top: 0.25rem;
                    min-width: 10rem;
                    z-index: 100;
                    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
                  `}
                >
                  <Flex direction="column" gap="xs">
                    <Text
                      tag="a"
                      href={messages.audiusUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      variant="body"
                      strength="strong"
                      onClick={() => setNavMenuOpen(false)}
                      css={css`
                        display: block;
                        padding: 0.5rem;
                        color: inherit;
                        text-decoration: none;
                        &:hover {
                          text-decoration: underline;
                          background: var(--harmony-neutral-neutral-2, #f0f0f0);
                          margin: 0 -0.5rem;
                          padding: 0.5rem;
                        }
                      `}
                    >
                      {messages.navAudius}
                    </Text>
                    <Text
                      tag="a"
                      href={`https://${messages.apiDocsUrl}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      variant="body"
                      strength="strong"
                      onClick={() => setNavMenuOpen(false)}
                      css={css`
                        display: block;
                        padding: 0.5rem;
                        color: inherit;
                        text-decoration: none;
                        &:hover {
                          text-decoration: underline;
                          background: var(--harmony-neutral-neutral-2, #f0f0f0);
                          margin: 0 -0.5rem;
                          padding: 0.5rem;
                        }
                      `}
                    >
                      {messages.navApiDocs}
                    </Text>
                    <Text
                      tag="a"
                      href={messages.githubUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      variant="body"
                      strength="strong"
                      onClick={() => setNavMenuOpen(false)}
                      css={css`
                        display: block;
                        padding: 0.5rem;
                        color: inherit;
                        text-decoration: none;
                        &:hover {
                          text-decoration: underline;
                          background: var(--harmony-neutral-neutral-2, #f0f0f0);
                          margin: 0 -0.5rem;
                          padding: 0.5rem;
                        }
                      `}
                    >
                      {messages.navGithub}
                    </Text>
                    <Text
                      tag="a"
                      href={messages.discordUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      variant="body"
                      strength="strong"
                      onClick={() => setNavMenuOpen(false)}
                      css={css`
                        display: block;
                        padding: 0.5rem;
                        color: inherit;
                        text-decoration: none;
                        &:hover {
                          text-decoration: underline;
                          background: var(--harmony-neutral-neutral-2, #f0f0f0);
                          margin: 0 -0.5rem;
                          padding: 0.5rem;
                        }
                      `}
                    >
                      {messages.navDiscord}
                    </Text>
                  </Flex>
                </Paper>
              ) : null}
            </span>
          </Flex>
          {oauthUser ? (
            <Button variant="secondary" size="small" onClick={handleLogout}>
              {messages.logout}
            </Button>
          ) : (
            <Button variant="primary" size="small" onClick={handleLogin}>
              {messages.login}
            </Button>
          )}
        </Flex>

        {/* Header */}
        <Flex
          direction="column"
          gap="s"
          alignItems="center"
          css={css`
            text-align: center;
          `}
        >
          <Text
            size="l"
            color="heading"
            strength="strong"
            variant="display"
            css={css`
              @media (max-width: 600px) {
                font-size: 3rem !important;
                line-height: 3rem !important;
              }
            `}
          >
            {messages.title}
          </Text>
          <Text
            color="default"
            variant="body"
            size="l"
            css={css`
              max-width: 40rem;
              margin: 0 auto;
              @media (max-width: 600px) {
                font-size: 0.875rem !important;
              }
            `}
          >
            Bring music to all your apps. Vibe-code ready and performant access
            to the world's largest open music catalog, the&nbsp;
            <span ref={openAudioLinkRef}>
              <Text
                tag="a"
                href="https://openaudio.org"
                target="_blank"
                rel="noopener noreferrer"
                onMouseEnter={(e) => {
                  grayscaleOrigin.current = { x: e.clientX, y: e.clientY };
                  setGrayscaleExiting(false);
                  setGrayscaleHover(true);
                }}
                onMouseLeave={() => setGrayscaleExiting(true)}
                css={css`
                  color: inherit;
                  text-decoration: underline;
                  cursor: pointer;
                  &:hover {
                    text-decoration: none;
                  }
                `}
              >
                Open Audio Protocol
              </Text>
            </span>
          </Text>
        </Flex>

        {/* Plans Section */}
        <Flex
          gap="l"
          css={css`
            @media (max-width: 768px) {
              flex-direction: column;
            }
          `}
        >
          {/* Free Plan */}
          <Paper
            p="xl"
            css={css`
              flex: 1;
              border-radius: 12px;
              box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
            `}
          >
            <Flex direction="column" gap="m">
              <Flex gap="s" alignItems="center">
                <FreePlanGraphic />
                <Text color="heading" strength="strong" variant="heading">
                  {messages.freePlan}
                </Text>
              </Flex>
              <Text color="subdued" variant="body">
                {messages.freeSubtitle}
              </Text>
              <Flex direction="column" gap="s">
                <Text variant="label" strength="strong" color="subdued">
                  {messages.usageDetails}
                </Text>
                <Flex gap="s" alignItems="center">
                  <IconValidationCheck size="s" />
                  <Text variant="body">
                    {messages.freeRateLimit} {messages.requestsPerSecond}
                  </Text>
                </Flex>
                <Flex gap="s" alignItems="center">
                  <IconValidationCheck size="s" />
                  <Text variant="body">
                    {messages.freeMonthlyLimit} {messages.requestsPerMonth}
                  </Text>
                </Flex>
              </Flex>
              <Button
                variant="primary"
                onClick={handleLogin}
                disabled={!!oauthUser}
                css={css`
                  margin-top: 0.5rem;
                `}
              >
                {messages.createApiKey}
              </Button>
            </Flex>
          </Paper>

          {/* Unlimited Plan */}
          <Paper
            p="xl"
            css={css`
              flex: 1;
              border-radius: 12px;
              box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
              background: var(--harmony-neutral-neutral-1, #fafafa);
              border: 1px solid var(--harmony-neutral-neutral-3, #e8e8e8);
            `}
          >
            <Flex direction="column" gap="m">
              <Flex gap="s" alignItems="center">
                <UnlimitedPlanGraphic />
                <Text color="heading" strength="strong" variant="heading">
                  {messages.unlimitedPlan}
                </Text>
              </Flex>
              <Text color="subdued" variant="body">
                {messages.unlimitedSubtitle}
              </Text>
              <Flex direction="column" gap="s">
                <Text variant="label" strength="strong" color="subdued">
                  {messages.usageDetails}
                </Text>
                <Flex gap="s" alignItems="center">
                  <IconValidationCheck size="s" />
                  <Text variant="body">
                    {messages.unlimitedRateLimit} {messages.requestsPerSecond}
                  </Text>
                </Flex>
                <Flex gap="s" alignItems="center">
                  <IconValidationCheck size="s" />
                  <Text variant="body">
                    {messages.unlimitedMonthlyLimit} {messages.requestsPerMonth}
                  </Text>
                </Flex>
              </Flex>
              <Flex
                p="m"
                direction="column"
                gap="xs"
                css={css`
                  margin-top: 0.5rem;
                  background: var(--harmony-background, #fff);
                  border-radius: 8px;
                  border: 1px solid var(--harmony-neutral-neutral-3, #e8e8e8);
                `}
              >
                <Text variant="label" strength="strong">
                  {messages.contactUs}
                </Text>
                <TextLink
                  href={`mailto:${messages.contactEmail}`}
                  variant="default"
                  isExternal
                >
                  {messages.contactEmail}
                </TextLink>
              </Flex>
            </Flex>
          </Paper>
        </Flex>

        {oauthUser ? (
          /* Your API Keys Section */
          <Flex direction="column" gap="s">
            <Flex
              justifyContent="space-between"
              alignItems="center"
              wrap="wrap"
              gap="s"
            >
              <Text
                size="s"
                color="heading"
                strength="strong"
                variant="display"
              >
                {messages.apiKeysSection}
              </Text>
              <Button
                variant="primary"
                size="small"
                onClick={() => setCreateKeyModalOpen(true)}
                iconLeft={IconPlus}
              >
                {messages.createNewKey}
              </Button>
            </Flex>
            <Paper
              p="xl"
              css={css`
                width: 100%;
              `}
            >
              <Flex
                direction="column"
                gap="m"
                css={css`
                  width: 100%;
                `}
              >
                <Flex gap="m" alignItems="center">
                  <ProfileAvatar
                    profilePicture={fullUser?.profilePicture}
                    size="medium"
                  />
                  <Flex direction="column" gap="xs">
                    <Text strength="strong">
                      {fullUser?.name ?? oauthUser.handle}
                    </Text>
                    <Text color="subdued">@{oauthUser.handle}</Text>
                  </Flex>
                </Flex>
                {developerApps.length === 0 ? (
                  <Text color="subdued">{messages.noDeveloperApps}</Text>
                ) : (
                  <Flex
                    direction="column"
                    gap="m"
                    css={css`
                      width: 100%;
                    `}
                  >
                    {developerApps.map((app) => (
                      <Paper key={app.address} p="l">
                        <Flex direction="column" gap="m" w="100%">
                          <Flex
                            gap="m"
                            alignItems="flex-start"
                            justifyContent="space-between"
                            wrap="wrap"
                            css={css`
                              @media (max-width: 600px) {
                                flex-direction: column;
                                align-items: stretch;
                              }
                            `}
                          >
                            <Flex
                              gap="m"
                              alignItems="flex-start"
                              css={css`
                                min-width: 260px;
                                width: 260px;
                                flex-shrink: 0;
                                @media (max-width: 600px) {
                                  width: 100%;
                                  min-width: 0;
                                }
                              `}
                            >
                              <AppAvatar app={app} />
                              <Flex
                                direction="column"
                                gap="xs"
                                style={{ minWidth: 0, flex: 1 }}
                              >
                                <Flex gap="s" alignItems="center">
                                  <Text strength="strong">{app.name}</Text>
                                  {app.is_legacy ? (
                                    <Tooltip
                                      text={messages.legacyTooltip}
                                      placement="top"
                                    >
                                      <span
                                        css={css`
                                          user-select: none;
                                          cursor: default;
                                          flex-shrink: 0;
                                          white-space: nowrap;
                                          &,
                                          &:hover,
                                          & *,
                                          & *:hover {
                                            cursor: default !important;
                                          }
                                        `}
                                      >
                                        <Tag>{messages.legacyPill}</Tag>
                                      </span>
                                    </Tooltip>
                                  ) : null}
                                  <span
                                    role="button"
                                    tabIndex={0}
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      setDeleteAppModalApp(app);
                                    }}
                                    onKeyDown={(e) => {
                                      if (e.key === "Enter" || e.key === " ") {
                                        e.preventDefault();
                                        setDeleteAppModalApp(app);
                                      }
                                    }}
                                    css={css`
                                      display: inline-flex;
                                      cursor: pointer;
                                    `}
                                  >
                                    <Tooltip
                                      text={messages.deleteApp}
                                      placement="top"
                                    >
                                      <IconButton
                                        icon={IconTrash}
                                        size="s"
                                        color="danger"
                                        aria-label={messages.deleteApp}
                                      />
                                    </Tooltip>
                                  </span>
                                </Flex>
                                {app.description ? (
                                  <Text color="subdued" variant="body">
                                    {app.description}
                                  </Text>
                                ) : null}
                              </Flex>
                            </Flex>
                            <Flex
                              direction="column"
                              gap="xs"
                              alignItems="flex-end"
                              css={css`
                                flex-shrink: 0;
                                margin-left: auto;
                                @media (max-width: 600px) {
                                  align-items: flex-start;
                                  margin-left: 0;
                                }
                              `}
                            >
                              <Text variant="body" color="subdued">
                                {messages.requestsThisMonth}:{" "}
                                {(app.request_count ?? 0).toLocaleString()}
                              </Text>
                              <Text variant="body" color="subdued">
                                {messages.requestsAllTime}:{" "}
                                {(
                                  app.request_count_all_time ?? 0
                                ).toLocaleString()}
                              </Text>
                            </Flex>
                          </Flex>
                          <Flex
                            direction="column"
                            gap="m"
                            css={css`
                              margin-top: 0.25rem;
                              padding-top: 0.75rem;
                              border-top: 1px solid
                                var(--harmony-neutral-neutral-3, #e8e8e8);
                            `}
                          >
                            <Flex gap="m" wrap="wrap">
                              <CopyableField
                                label={messages.apiKeyLabel}
                                value={app.address}
                                id={`apikey-${app.address}`}
                                compact
                              />
                            </Flex>
                            {!app.is_legacy &&
                            (app.api_access_keys?.length ?? 0) > 0
                              ? (app.api_access_keys
                                  ?.filter((aak) => aak.is_active !== false)
                                  ?.map((aak, idx) => (
                                    <BearerTokenField
                                      key={`${app.address}-${aak.api_access_key}`}
                                      value={aak.api_access_key}
                                      id={`apisecret-${app.address}-${idx}`}
                                      onRevoke={() =>
                                        handleDeactivateAccessKey(
                                          app,
                                          aak.api_access_key,
                                        )
                                      }
                                      onAdd={() => handleAddAccessKey(app)}
                                    />
                                  )) ?? null)
                              : null}
                          </Flex>
                        </Flex>
                      </Paper>
                    ))}
                  </Flex>
                )}
              </Flex>
            </Paper>
          </Flex>
        ) : null}

        {/* Getting Started Section */}
        <Flex
          direction="column"
          gap="s"
          css={css`
            width: 100%;
          `}
        >
          <Text size="s" color="heading" strength="strong" variant="display">
            {messages.quickstart}
          </Text>
          <Paper
            p="xl"
            css={css`
              width: 100%;
            `}
          >
            <Flex
              direction="column"
              gap="xl"
              css={css`
                width: 100%;
              `}
            >
              {/* Documentation Links */}
              <Flex direction="column" gap="m">
                <Text variant="label" strength="strong" color="subdued">
                  DOCUMENTATION
                </Text>
                <Flex direction="column" gap="s">
                  <Flex gap="s" alignItems="center" wrap="wrap">
                    <Text strength="strong">{messages.apiDocs}</Text>
                    <Text
                      tag="a"
                      href={`https://${messages.apiDocsUrl}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      css={css`
                        color: var(--harmony-primary-primary, #7d2fe0);
                        text-decoration: underline;
                        &:hover {
                          text-decoration: none;
                        }
                      `}
                    >
                      {messages.apiDocsUrl}
                    </Text>
                  </Flex>
                  <Flex gap="s" alignItems="center" wrap="wrap">
                    <Text strength="strong">{messages.swaggerDefinition}</Text>
                    <Text
                      tag="a"
                      href={`https://${messages.swaggerUrl}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      css={css`
                        color: var(--harmony-primary-primary, #7d2fe0);
                        text-decoration: underline;
                        &:hover {
                          text-decoration: none;
                        }
                      `}
                    >
                      {messages.swaggerUrl}
                    </Text>
                  </Flex>
                </Flex>
              </Flex>

              {/* REST API */}
              <Flex
                direction="column"
                gap="m"
                css={css`
                  padding-top: 1.5rem;
                  border-top: 1px solid
                    var(--harmony-neutral-neutral-3, #e8e8e8);
                `}
              >
                <Flex gap="s" alignItems="center" wrap="wrap">
                  <Text variant="label" strength="strong" color="subdued">
                    REST
                  </Text>
                  <Text
                    tag="a"
                    href={`https://${messages.restEndpoint}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    css={css`
                      color: var(--harmony-primary-primary, #7d2fe0);
                      text-decoration: underline;
                      font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace;
                      font-size: 0.875rem;
                      &:hover {
                        text-decoration: none;
                      }
                    `}
                  >
                    {messages.restEndpoint}
                  </Text>
                  <Flex
                    gap="xs"
                    alignItems="center"
                    css={css`
                      padding: 0.25rem 0.5rem;
                      border-radius: 4px;
                      background: rgba(34, 197, 94, 0.15);
                      color: #16a34a;
                      font-size: 0.75rem;
                      font-weight: 600;
                    `}
                  >
                    <IconValidationCheck size="xs" />
                    <Text variant="label" strength="strong" color="inherit">
                      {messages.online}
                    </Text>
                  </Flex>
                </Flex>
                <CopyableCodeBlock
                  code={`curl -X GET "https://api.audius.co/v1/tracks/trending"
  -H "Authorization: Bearer <YOUR-API-BEARER-TOKEN>"`}
                />
              </Flex>

              {/* GRPC */}
              <Flex
                direction="column"
                gap="m"
                css={css`
                  padding-top: 1.5rem;
                  border-top: 1px solid
                    var(--harmony-neutral-neutral-3, #e8e8e8);
                `}
              >
                <Flex gap="s" alignItems="center" wrap="wrap">
                  <Text variant="label" strength="strong" color="subdued">
                    GRPC
                  </Text>
                  <Text
                    css={css`
                      font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace;
                      font-size: 0.875rem;
                    `}
                  >
                    {messages.grpcEndpoint}
                  </Text>
                  <Flex
                    gap="xs"
                    alignItems="center"
                    css={css`
                      padding: 0.25rem 0.5rem;
                      border-radius: 4px;
                      background: rgba(34, 197, 94, 0.15);
                      color: #16a34a;
                      font-size: 0.75rem;
                      font-weight: 600;
                    `}
                  >
                    <IconValidationCheck size="xs" />
                    <Text variant="label" strength="strong" color="inherit">
                      {messages.online}
                    </Text>
                  </Flex>
                </Flex>
                <CopyableCodeBlock
                  code={`grpcurl -H "authorization: Bearer <YOUR-API-BEARER-TOKEN>" grpc.audius.co:443 list`}
                />
              </Flex>
            </Flex>
          </Paper>
        </Flex>

        {/* Footer */}
        <Flex
          direction="row"
          gap="l"
          justifyContent="flex-end"
          css={css`
            padding-top: 4rem;
            margin-top: 2rem;
            border-top: 1px solid var(--harmony-neutral-neutral-3, #e8e8e8);
          `}
        >
          <Text
            tag="a"
            href={messages.audiusUrl}
            target="_blank"
            rel="noopener noreferrer"
            variant="body"
            css={css`
              color: #999;
              text-decoration: none;
              &:hover {
                color: #666;
                text-decoration: underline;
              }
            `}
          >
            {messages.navAudius}
          </Text>
          <Text
            tag="a"
            href={`https://${messages.apiDocsUrl}`}
            target="_blank"
            rel="noopener noreferrer"
            variant="body"
            css={css`
              color: #999;
              text-decoration: none;
              &:hover {
                color: #666;
                text-decoration: underline;
              }
            `}
          >
            {messages.navApiDocs}
          </Text>
          <Text
            tag="a"
            href={messages.githubUrl}
            target="_blank"
            rel="noopener noreferrer"
            variant="body"
            css={css`
              color: #999;
              text-decoration: none;
              &:hover {
                color: #666;
                text-decoration: underline;
              }
            `}
          >
            {messages.navGithub}
          </Text>
          <Text
            tag="a"
            href={messages.discordUrl}
            target="_blank"
            rel="noopener noreferrer"
            variant="body"
            css={css`
              color: #999;
              text-decoration: none;
              &:hover {
                color: #666;
                text-decoration: underline;
              }
            `}
          >
            {messages.navDiscord}
          </Text>
        </Flex>
      </Flex>
    </HarmonyThemeProvider>
  );
}
