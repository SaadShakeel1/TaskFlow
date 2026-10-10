package main

import (
	"context"
	"fmt"

	"github.com/SaadShakeel1/TaskFlow/internal/mailer"
	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

func (app *application) enqueueTaskAssignmentNotification(
	ctx context.Context,
	project models.Project,
	task models.Task,
) {
	if task.AssigneeID == nil || app.notifications == nil {
		return
	}

	assignee, err := app.users.GetByID(ctx, *task.AssigneeID)
	if err != nil {
		app.errorLog.Printf(
			"load assignee %d for task %d notification: %v",
			*task.AssigneeID,
			task.ID,
			err,
		)
		return
	}

	email := mailer.Email{
		To:      assignee.Email,
		Subject: "You were assigned a TaskFlow task",
		Body: fmt.Sprintf(
			"Hi %s,\n\nYou were assigned the task %q in project %q.\n",
			assignee.Name,
			task.Title,
			project.Name,
		),
	}

	if err := app.notifications.Enqueue(email); err != nil {
		app.errorLog.Printf(
			"enqueue assignment notification for task %d: %v",
			task.ID,
			err,
		)
	}
}
