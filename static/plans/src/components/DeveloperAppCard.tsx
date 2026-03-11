import { useState } from "react";
import {
  Avatar,
  Text,
  Paper,
  Flex,
  Button,
  IconTrash,
  IconButton,
  Tag,
  Tooltip,
  IconClose,
  IconPlus,
} from "@audius/harmony";
import { css } from "@emotion/react";

export type ApiAccessKey = {
  api_access_key: string;
  is_active: boolean;
};

export type DeveloperApp = {
  address: string;
  user_id: string;
  name: string;
  description: string | null;
  image_url: string | null;
  request_count?: number;
  request_count_all_time?: number;
  is_legacy?: boolean;
  api_access_keys?: ApiAccessKey[];
  redirect_uris?: string[];
};

const messages = {
  requestsThisMonth: "Requests this month",
  requestsAllTime: "Requests all time",
  apiKeyLabel: "API Key",
  apiSecretLabel: "API Bearer Token",
  copy: "Copy",
  copied: "Copied!",
  legacyPill: "LEGACY app",
  legacyTooltip:
    "Sign API requests using your API Secret Key in the Authorization Header",
  deleteApp: "Delete API Key",
  revokeToken: "Revoke Bearer Token",
  newBearerToken: "New Bearer Token",
  redirectUrisLabel: "Redirect URIs",
  removeUri: "Remove Redirect URI",
  addUri: "Add Redirect URI",
  addUriPlaceholder: "https://example.com/callback",
};

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
        <Tooltip text={messages.revokeToken} placement="top">
          <IconButton
            icon={IconClose}
            size="s"
            color="danger"
            aria-label={messages.revokeToken}
            disabled={pending}
            onClick={(e) => {
              e.stopPropagation();
              void handleRevoke();
            }}
          />
        </Tooltip>
        <Tooltip text={messages.newBearerToken} placement="top">
          <IconButton
            icon={IconPlus}
            size="s"
            color="success"
            aria-label={messages.newBearerToken}
            disabled={pending}
            onClick={(e) => {
              e.stopPropagation();
              void handleAdd();
            }}
          />
        </Tooltip>
      </Flex>
    </Flex>
  );
}

function RedirectUriRow({
  value,
  onRemove,
}: {
  value: string;
  onRemove: () => void | Promise<void>;
}) {
  const [pending, setPending] = useState(false);

  const handleRemove = async () => {
    if (pending) return;
    setPending(true);
    try {
      await onRemove();
    } catch (err) {
      console.error(err);
    } finally {
      setPending(false);
    }
  };

  return (
    <Flex gap="s" alignItems="center">
      <input
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
      <Tooltip text={messages.removeUri} placement="top">
        <IconButton
          icon={IconClose}
          size="s"
          color="danger"
          aria-label={messages.removeUri}
          disabled={pending}
          onClick={handleRemove}
        />
      </Tooltip>
    </Flex>
  );
}

function AddRedirectUriRow({
  onAdd,
}: {
  onAdd: (uri: string) => void | Promise<void>;
}) {
  const [value, setValue] = useState("");
  const [pending, setPending] = useState(false);

  const handleAdd = async () => {
    if (pending || !value.trim()) return;
    setPending(true);
    try {
      await onAdd(value.trim());
      setValue("");
    } catch (err) {
      console.error(err);
    } finally {
      setPending(false);
    }
  };

  return (
    <Flex gap="s" alignItems="center">
      <input
        type="text"
        placeholder={messages.addUriPlaceholder}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") void handleAdd();
        }}
        css={css`
          flex: 1;
          padding: 0.5rem 0.75rem;
          font-family: "Monaco", "Menlo", "Ubuntu Mono", monospace;
          font-size: 0.875rem;
          border: 1px solid var(--harmony-neutral-neutral-3, #e0e0e0);
          border-radius: 4px;
          background: white;
          min-width: 0;
        `}
      />
      <Tooltip text={messages.addUri} placement="top">
        <IconButton
          icon={IconPlus}
          size="s"
          color="success"
          aria-label={messages.addUri}
          disabled={pending || !value.trim()}
          onClick={handleAdd}
        />
      </Tooltip>
    </Flex>
  );
}

function RedirectUrisField({
  uris,
  appAddress,
  onRemove,
  onAdd,
}: {
  uris: string[];
  appAddress: string;
  onRemove: (uri: string) => void | Promise<void>;
  onAdd: (uri: string) => void | Promise<void>;
}) {
  return (
    <Flex direction="column" gap="xs">
      <Text tag="label" variant="body" strength="strong">
        {messages.redirectUrisLabel}
      </Text>
      {uris.map((uri) => (
        <RedirectUriRow
          key={`${appAddress}-uri-${uri}`}
          value={uri}
          onRemove={() => onRemove(uri)}
        />
      ))}
      <AddRedirectUriRow onAdd={onAdd} />
    </Flex>
  );
}

type Props = {
  app: DeveloperApp;
  onDelete: (app: DeveloperApp) => void;
  onDeactivateAccessKey: (app: DeveloperApp, apiAccessKey: string) => void;
  onAddAccessKey: (app: DeveloperApp) => void;
  onAddRedirectUri: (app: DeveloperApp, uri: string) => void;
  onRemoveRedirectUri: (app: DeveloperApp, uri: string) => void;
};

export function DeveloperAppCard({
  app,
  onDelete,
  onDeactivateAccessKey,
  onAddAccessKey,
  onAddRedirectUri,
  onRemoveRedirectUri,
}: Props) {
  return (
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
            <Flex direction="column" gap="xs" style={{ minWidth: 0, flex: 1 }}>
              <Flex gap="s" alignItems="center">
                <Text strength="strong">{app.name}</Text>
                {app.is_legacy ? (
                  <Tooltip text={messages.legacyTooltip} placement="top">
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
                <Tooltip text={messages.deleteApp} placement="top">
                  <IconButton
                    icon={IconTrash}
                    size="s"
                    aria-label={messages.deleteApp}
                    onClick={(e) => {
                      e.stopPropagation();
                      onDelete(app);
                    }}
                    css={css`
                      display: inline-flex;
                    `}
                  />
                </Tooltip>
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
              {(app.request_count_all_time ?? 0).toLocaleString()}
            </Text>
          </Flex>
        </Flex>
        <Flex
          direction="column"
          gap="m"
          css={css`
            margin-top: 0.25rem;
            padding-top: 0.75rem;
            border-top: 1px solid var(--harmony-neutral-neutral-3, #e8e8e8);
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
          {!app.is_legacy && (app.api_access_keys?.length ?? 0) > 0
            ? app.api_access_keys
                ?.filter((aak) => aak.is_active !== false)
                ?.map((aak, idx) => (
                  <BearerTokenField
                    key={`${app.address}-${aak.api_access_key}`}
                    value={aak.api_access_key}
                    id={`apisecret-${app.address}-${idx}`}
                    onRevoke={() =>
                      onDeactivateAccessKey(app, aak.api_access_key)
                    }
                    onAdd={() => onAddAccessKey(app)}
                  />
                )) ?? null
            : null}
          {!app.is_legacy ? (
            <RedirectUrisField
              uris={app.redirect_uris ?? []}
              appAddress={app.address}
              onRemove={(uri) => onRemoveRedirectUri(app, uri)}
              onAdd={(uri) => onAddRedirectUri(app, uri)}
            />
          ) : null}
        </Flex>
      </Flex>
    </Paper>
  );
}
