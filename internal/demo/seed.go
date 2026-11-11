// Package demo seeds explicitly fictional content for local UI acceptance only.
package demo

import (
	"context"
	"time"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
	"YuLing_senior_high_school/internal/web"
)

func Seed(store *dao.MemoryStore) error {
	ctx := context.Background()
	people := []model.Alumni{
		{
			Name: "王知远", GraduationYear: 2012, ClassName: "高三2班", City: "西安",
			University: "西安交通大学", Major: "电气工程", Company: "能源科技", Position: "工程师", AvatarColor: "blue",
		},
		{
			Name: "李书宁", GraduationYear: 2015, ClassName: "高三1班", City: "北京",
			University: "北京大学", Major: "中文", Company: "文化出版", Position: "编辑", AvatarColor: "pink",
		},
		{
			Name: "张景行", GraduationYear: 2018, ClassName: "高三3班", City: "西安",
			University: "西北工业大学", Major: "航空航天", Company: "航空研究", Position: "研究员", AvatarColor: "green",
		},
		{
			Name: "高予安", GraduationYear: 2012, ClassName: "高三2班", City: "上海",
			University: "上海交通大学", Major: "计算机科学", Company: "互联网", Position: "工程师", AvatarColor: "gold",
		},
		{
			Name: "马明轩", GraduationYear: 2020, ClassName: "高三5班", City: "成都",
			University: "电子科技大学", Major: "电子信息", AvatarColor: "red",
		},
		{
			Name: "刘诗语", GraduationYear: 2022, ClassName: "高三1班", City: "北京",
			University: "北京师范大学", Major: "教育学", AvatarColor: "pink",
		},
		{
			Name: "白云舟", GraduationYear: 2018, ClassName: "高三3班", City: "西安",
			University: "西安交通大学", Major: "机械工程", Company: "智能制造", AvatarColor: "blue",
		},
		{
			Name: "赵清和", GraduationYear: 2024, ClassName: "高三6班", City: "西安",
			University: "西安电子科技大学", Major: "通信工程", AvatarColor: "green",
		},
		{
			Name: "常思齐", GraduationYear: 2026, ClassName: "高三2班", City: "西安",
			University: "西北工业大学", Major: "自动化", AvatarColor: "gold",
		},
		{
			Name: "何沐阳", GraduationYear: time.Now().Year() + 1, ClassName: "高三1班", City: "榆林",
			Identity: "student", DreamUniversity: "北京大学", AvatarColor: "red",
		},
		{
			Name: "苏若溪", GraduationYear: time.Now().Year() + 1, ClassName: "高三1班", City: "榆林",
			Identity: "student", DreamUniversity: "西安交通大学", AvatarColor: "pink",
		},
		{
			Name: "陈星野", GraduationYear: 2016, ClassName: "高三4班", City: "杭州",
			University: "浙江大学", Major: "建筑学", Company: "建筑设计", Position: "建筑师", AvatarColor: "green",
		},
		{
			Name: "周亦然", GraduationYear: 2019, ClassName: "高三2班", City: "西安",
			University: "西安交通大学", Major: "管理科学", Company: "科技服务", Position: "产品经理", AvatarColor: "blue",
		},
		{
			Name: "林见夏", GraduationYear: 2021, ClassName: "高三1班", City: "北京",
			University: "北京大学", Major: "新闻传播", Company: "传媒", AvatarColor: "gold",
		},
		{
			Name: "杨怀瑾", GraduationYear: 2018, ClassName: "高三3班", City: "西安",
			University: "西北工业大学", Major: "材料科学", Company: "科研院所", Position: "工程师", AvatarColor: "red",
		},
	}
	for _, person := range people {
		if person.Identity == "" {
			person.Identity = "alumni"
		}
		person.Status = "active"
		person.Message = "山河远阔，愿我们都成为更好的自己。期待与老同学再相逢。"
		if err := store.CreateAlumni(ctx, &person); err != nil {
			return err
		}
	}
	photos := []struct{ file, caption, category string }{
		{file: "campus-old.jpg", caption: "记忆里的校门，还是最熟悉的模样", category: "老照片"},
		{file: "campus-detail.jpg", caption: "从前的校园，珍藏在时光里", category: "老照片"},
		{file: "campus-main.jpg", caption: "榆中校园的一抹秋色", category: "校园风景"},
	}
	for _, photo := range photos {
		data, err := web.DemoImage(photo.file)
		if err != nil {
			return err
		}
		row := model.Photo{Caption: photo.caption, Uploader: "校园记忆（演示）", Category: photo.category, Image: data, MIME: "image/jpeg", Status: "active"}
		if err := store.CreatePhoto(ctx, &row); err != nil {
			return err
		}
	}
	posts := []model.Post{
		{Nickname: "知远同学", GraduationYear: 2012, Content: "这些年走过很多地方，最想念的还是榆中的课间走廊。有没有同学还留着当年的毕业合影？一起把那些回忆找回来吧。", AvatarColor: "blue", Status: "active"},
		{Nickname: "榆中小林", GraduationYear: 2021, Content: "发现这里有好多熟悉的大学名字！在读的学弟学妹如果想了解大学生活，欢迎在这里交流。愿大家都能奔赴心里的山海。", AvatarColor: "gold", Status: "active"},
		{
			Nickname: "杨怀瑾", GraduationYear: 2018, AvatarColor: "red", Status: "active",
			Content: "工作后才懂得，同窗这两个字有多珍贵。\n" +
				"还记得那年晚自习窗外的落日，和操场上一圈又一圈的奔跑。很高兴，在这里和大家再次相遇。",
		},
	}
	for _, post := range posts {
		if err := store.CreatePost(ctx, &post); err != nil {
			return err
		}
	}
	_, err := store.CreateReply(ctx, &model.Reply{PostID: 3, Nickname: "见夏", Content: "同窗如初，有空一起回母校看看！", Status: "active"})
	return err
}
