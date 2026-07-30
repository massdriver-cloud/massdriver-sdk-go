package types

// Ptr returns a pointer to v. Convenience for populating the optional
// pointer fields on update inputs, where nil means "leave unchanged":
//
//	c.Projects.Update(ctx, id, projects.UpdateInput{
//	    Description: types.Ptr("new description"),
//	})
func Ptr[T any](v T) *T { return &v }
