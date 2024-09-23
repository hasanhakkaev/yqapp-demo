// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: queries.sql

package database

import (
	"context"
)

const createTask = `-- name: CreateTask :one
INSERT INTO tasks (type, value, state, creation_time, last_update_time)
VALUES ($1, $2, $3, $4, $5)
RETURNING id
`

type CreateTaskParams struct {
	Type           uint32
	Value          uint32
	State          State
	CreationTime   float64
	LastUpdateTime float64
}

func (q *Queries) CreateTask(ctx context.Context, arg CreateTaskParams) (int32, error) {
	row := q.db.QueryRow(ctx, createTask,
		arg.Type,
		arg.Value,
		arg.State,
		arg.CreationTime,
		arg.LastUpdateTime,
	)
	var id int32
	err := row.Scan(&id)
	return id, err
}

const getSumOfTasksByState = `-- name: GetSumOfTasksByState :many
SELECT state, COUNT(*) AS task_count
FROM tasks
GROUP BY state
`

type GetSumOfTasksByStateRow struct {
	State     State
	TaskCount int64
}

func (q *Queries) GetSumOfTasksByState(ctx context.Context) ([]GetSumOfTasksByStateRow, error) {
	rows, err := q.db.Query(ctx, getSumOfTasksByState)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetSumOfTasksByStateRow
	for rows.Next() {
		var i GetSumOfTasksByStateRow
		if err := rows.Scan(&i.State, &i.TaskCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getSumOfValues = `-- name: GetSumOfValues :many
SELECT type, SUM(value) AS total_value
FROM tasks
GROUP BY type
`

type GetSumOfValuesRow struct {
	Type       uint32
	TotalValue int64
}

func (q *Queries) GetSumOfValues(ctx context.Context) ([]GetSumOfValuesRow, error) {
	rows, err := q.db.Query(ctx, getSumOfValues)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetSumOfValuesRow
	for rows.Next() {
		var i GetSumOfValuesRow
		if err := rows.Scan(&i.Type, &i.TotalValue); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getTasksByState = `-- name: GetTasksByState :many
SELECT id, type, value, state, creation_time, last_update_time
FROM tasks
WHERE state = $1
`

func (q *Queries) GetTasksByState(ctx context.Context, state State) ([]Task, error) {
	rows, err := q.db.Query(ctx, getTasksByState, state)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Task
	for rows.Next() {
		var i Task
		if err := rows.Scan(
			&i.ID,
			&i.Type,
			&i.Value,
			&i.State,
			&i.CreationTime,
			&i.LastUpdateTime,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateTaskState = `-- name: UpdateTaskState :one
UPDATE tasks
SET state = $1, last_update_time = $2
WHERE id = $3
RETURNING id, type, value, state, creation_time, last_update_time
`

type UpdateTaskStateParams struct {
	State          State
	LastUpdateTime float64
	ID             int32
}

func (q *Queries) UpdateTaskState(ctx context.Context, arg UpdateTaskStateParams) (Task, error) {
	row := q.db.QueryRow(ctx, updateTaskState, arg.State, arg.LastUpdateTime, arg.ID)
	var i Task
	err := row.Scan(
		&i.ID,
		&i.Type,
		&i.Value,
		&i.State,
		&i.CreationTime,
		&i.LastUpdateTime,
	)
	return i, err
}
