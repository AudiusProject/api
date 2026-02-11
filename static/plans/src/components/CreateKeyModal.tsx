import { useState, useCallback } from "react";
import {
  Modal,
  ModalHeader,
  ModalTitle,
  ModalContent,
  ModalFooter,
  Button,
  Flex,
  TextInput,
  IconEmbed,
} from "@audius/harmony";
import { css } from "@emotion/react";

const messages = {
  title: "Create New Key",
  nameLabel: "Key name",
  namePlaceholder: "Enter a name for your API key",
  cancel: "Cancel",
  create: "Create",
  creating: "Creating...",
};

type CreateKeyModalProps = {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (name: string) => Promise<void>;
};

export const CreateKeyModal = ({
  isOpen,
  onClose,
  onSubmit,
}: CreateKeyModalProps) => {
  const [name, setName] = useState("");
  const [isPending, setIsPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = useCallback(async () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    setIsPending(true);
    setError(null);
    try {
      await onSubmit(trimmed);
      setName("");
      onClose();
    } catch (e) {
      setError((e as Error)?.message ?? "Something went wrong");
    } finally {
      setIsPending(false);
    }
  }, [name, onSubmit, onClose]);

  const handleClose = useCallback(() => {
    if (!isPending) {
      setName("");
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
          /* Put close button and title on the same line */
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
          <ModalTitle title={messages.title} Icon={IconEmbed} />
        </ModalHeader>
      </div>
      <ModalContent>
        <Flex
          direction="column"
          gap="m"
          css={css`
            min-width: 0;
            & input,
            & label {
              font-family: var(--harmony-font-family);
              overflow-x: hidden;
              min-width: 0;
            }
          `}
        >
          <TextInput
            label={messages.nameLabel}
            placeholder={messages.namePlaceholder}
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={isPending}
          />
          {error ? (
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
            variant="primary"
            onClick={handleSubmit}
            disabled={!name.trim() || isPending}
            isLoading={isPending}
          >
            {isPending ? messages.creating : messages.create}
          </Button>
        </Flex>
      </ModalFooter>
    </Modal>
  );
};
