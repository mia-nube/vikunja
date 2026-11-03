import {useMutation} from '@tanstack/vue-query'
import {
	taskAttachmentsCreateLink,
	taskAttachmentsDelete,
	taskAttachmentsDownload,
	taskAttachmentsPreview,
	taskAttachmentsUpload,
} from '@/client/generated'
import type {
	TaskAttachment,
	TaskAttachmentsPreviewData,
} from '@/client/generated'
import {i18n} from '@/i18n'
import {expectBlob} from './blobResponse'
import {contextMutationOptions} from './contextMutation'
import {
	invalidateTaskMembership,
	mapTaskEverywhere,
} from './taskCache'

// The task detail and every list response already carry `attachments`, so there is no list query.
export const attachmentKeys = {
	blobs: ['attachments', 'blob'] as const,
	blobsFor: (taskId: number, id: number) => [...attachmentKeys.blobs, taskId, id] as const,
	blob: (taskId: number, id: number, size?: PreviewSize) => [...attachmentKeys.blobsFor(taskId, id), size] as const,
}

export type PreviewSize = TaskAttachmentsPreviewData['query']['size']
export type AttachmentIdentity = Required<Pick<TaskAttachment, 'id' | 'task_id'>>

export async function attachmentBlob(attachment: AttachmentIdentity, size?: PreviewSize, signal?: AbortSignal) {
	const path = {
		task: attachment.task_id,
		attachment: attachment.id,
	}
	const {data} = size === undefined
		? await taskAttachmentsDownload({
			path,
			parseAs: 'blob',
			signal,
		})
		: await taskAttachmentsPreview({
			path,
			query: {size},
			parseAs: 'blob',
			signal,
		})
	return expectBlob(data, 'Attachment')
}

export function uploadAttachmentsMutationOptions(shouldNotify: () => boolean = () => true) {
	return contextMutationOptions({
		mutationFn: async ({taskId, files}: {
			taskId: number,
			files: File[],
		}) =>
			(await taskAttachmentsUpload({
				path: {task: taskId},
				body: {files},
			})).data,
		onSuccess: (result, {taskId}, client) => {
			const uploaded = result.success ?? []
			mapTaskEverywhere(client, taskId, task => ({
				...task,
				attachments: [
					...task.attachments.filter(item => !uploaded.some(attachment => attachment.id === item.id)),
					...uploaded,
				],
			}))
		},
		onSettled: ({taskId}, client) => invalidateTaskMembership(client, taskId),
		toastError: shouldNotify,
	})
}

// Modified by mia·nube on 2026-11-03: the create-link mutation, ported from the
// fork's 2026-08-03 AttachmentService.createLink (upstream v2.7 removed the
// services). It attaches a file held by an external system as a reference
// rather than uploading a copy. The request carries the user's own Vikunja
// credentials like every other call, so the server applies the same write
// permission an upload needs — the picker that produced the selection grants
// nothing on its own.
export function createLinkAttachmentMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({taskId, link}: {
			taskId: number,
			link: {
				provider: string,
				ref: string,
				name: string,
				size: number,
				mime: string,
			},
		}) =>
			(await taskAttachmentsCreateLink({
				path: {task: taskId},
				body: {
					link_provider: link.provider,
					link_ref: link.ref,
					link_name: link.name,
					link_size: link.size,
					link_mime: link.mime,
				},
			})).data,
		onSuccess: (created, {taskId}, client) => {
			mapTaskEverywhere(client, taskId, task => ({
				...task,
				attachments: [
					...task.attachments.filter(item => item.id !== created.id),
					created,
				],
			}))
		},
		onSettled: ({taskId}, client) => invalidateTaskMembership(client, taskId),
		successMessage: () => i18n.global.t('task.attachment.linkAttached'),
	})
}

export function useCreateLinkAttachmentMutation() {
	return useMutation(createLinkAttachmentMutationOptions())
}

export function deleteAttachmentMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({taskId, id}: {
			taskId: number,
			id: number,
		}) =>
			(await taskAttachmentsDelete({path: {
				task: taskId,
				attachment: id,
			}})).data,
		onSuccess: (_data, {taskId, id}, client) => {
			mapTaskEverywhere(client, taskId, task => ({
				...task,
				attachments: task.attachments.filter(a => a.id !== id),
				cover_image_attachment_id: task.cover_image_attachment_id === id ? 0 : task.cover_image_attachment_id,
			}))
			client.removeQueries({queryKey: attachmentKeys.blobsFor(taskId, id)})
		},
		onSettled: ({taskId}, client) => invalidateTaskMembership(client, taskId),
		successMessage: () => i18n.global.t('task.attachment.deleteSuccess'),
	})
}

export function useUploadAttachmentsMutation(shouldNotify?: () => boolean) {
	return useMutation(uploadAttachmentsMutationOptions(shouldNotify))
}

export function useDeleteAttachmentMutation() {
	return useMutation(deleteAttachmentMutationOptions())
}
