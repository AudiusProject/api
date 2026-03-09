import { useState, useCallback } from "react";
import {
  Modal,
  ModalHeader,
  ModalTitle,
  ModalContent,
  ModalFooter,
  Button,
  Flex,
  Text,
  IconTrash,
} from "@audius/harmony";
import { css } from "@emotion/react";

const messages = {
  title: "Delete API Key",
  description:
    "Are you sure you want to delete this API key? This action cannot be undone.",
  cancel: "Cancel",
  confirm: "Delete",
  deleting: "Deleting...",
};

type DeveloperApp = {
  address: string;
  name: string;
};

type DeleteAppModalProps = {
  isOpen: boolean;
  onClose: () => void;
  app: DeveloperApp | null;
  onConfirm: (app: DeveloperApp) => Promise<void>;
};

export const DeleteAppModal = ({
  isOpen,
  onClose,
  app,
  onConfirm,
}: DeleteAppModalProps) => {
  const [isPending, setIsPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleConfirm = useCallback(async () => {
    if (app == null) return;
    setIsPending(true);
    setError(null);
    try {
      await onConfirm(app);
      onClose();
    } catch (e) {
      setError((e as Error)?.message ?? "Something went wrong");
    } finally {
      setIsPending(false);
    }
  }, [app, onConfirm, onClose]);

  const handleClose = useCallback(() => {
    if (!isPending) {
      setError(null);
      onClose();
    }
  }, [isPending, onClose]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      size="small"
      css={css`
        font-family: var(--harmony-font-family);
      `}
    >
      <div
        css={css`
          & > div {
            flex-direction: row;
            flex-wrap: wrap;
            align-items: center;
            gap: var(--harmony-unit-2);
          }
          & > div > button {
            position: relative;
            flex-shrink: 0;
          }
          & > div > div {
            flex: 1;
            min-width: 0;
            margin-left: var(--harmony-unit-4);
            margin-right: var(--harmony-unit-4);
          }
        `}
      >
        <ModalHeader>
          <ModalTitle title={messages.title} Icon={IconTrash} />
        </ModalHeader>
      </div>
      <ModalContent>
        <Flex
          direction="column"
          gap="m"
          css={css`
            min-width: 0;
          `}
        >
          <Text variant="body" color="subdued">
            {messages.description}
          </Text>
          {app != null ? (
            <Text variant="body" strength="strong">
              {app.name}
            </Text>
          ) : null}
          {error != null ? (
            <Flex
              css={css`
                color: var(--harmony-red, #e53935);
                font-size: 0.875rem;
              `}
            >
              {error}
            </Flex>
          ) : null}
        </Flex>
      </ModalContent>
      <ModalFooter>
        <Flex gap="s" justifyContent="flex-end">
          <Button
            variant="secondary"
            onClick={handleClose}
            disabled={isPending}
          >
            {messages.cancel}
          </Button>
          <Button
            variant="destructive"
            onClick={handleConfirm}
            disabled={app == null || isPending}
            isLoading={isPending}
          >
            {isPending ? messages.deleting : messages.confirm}
          </Button>
        </Flex>
      </ModalFooter>
    </Modal>
  );
};
